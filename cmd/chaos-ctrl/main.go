package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/chaos"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

const (
	defaultNamespace = "cluster-probe"
)

var (
	version   = "dev"
	commitSHA = "unknown"
	buildDate = "unknown"
)

type chaosAPI interface {
	Apply(ctx context.Context, spec chaos.ExperimentSpec) (string, error)
	Status(ctx context.Context, id string) (chaos.ExperimentStatus, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]chaos.ExperimentStatus, error)
}

type commandConfig struct {
	namespace  string
	kubeconfig string
	context    string
	output     string
	file       string
	args       []string
	stdout     io.Writer
	stderr     io.Writer
	newClient  func(context.Context, commandConfig) (chaosAPI, error)
	readFile   func(string) ([]byte, error)
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, newChaosClient); err != nil {
		fmt.Fprintf(os.Stderr, "chaos-ctrl: %v\n", err)
		os.Exit(1)
	}
}

func run(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	newClient func(context.Context, commandConfig) (chaosAPI, error),
) error {
	cfg, command, err := parseArgs(args, stdout, stderr)
	if err != nil {
		return err
	}
	cfg.stdout = stdout
	cfg.stderr = stderr
	cfg.newClient = newClient
	cfg.readFile = os.ReadFile

	if command == "version" {
		return writeOutput(stdout, cfg.output, map[string]string{
			"version":    version,
			"commit_sha": commitSHA,
			"build_date": buildDate,
		})
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := cfg.newClient(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create chaos client: %w", err)
	}

	switch command {
	case "apply":
		return runApply(ctx, cfg, client)
	case "status":
		return runStatus(ctx, cfg, client)
	case "delete":
		return runDelete(ctx, cfg, client)
	case "list":
		return runList(ctx, cfg, client)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func parseArgs(args []string, stdout io.Writer, stderr io.Writer) (commandConfig, string, error) {
	cfg := commandConfig{
		namespace: defaultNamespace,
		output:    "text",
	}

	flags := flag.NewFlagSet("chaos-ctrl", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&cfg.namespace, "namespace", defaultNamespace, "Chaos Mesh namespace")
	flags.StringVar(&cfg.kubeconfig, "kubeconfig", "", "path to kubeconfig; defaults to in-cluster config or KUBECONFIG")
	flags.StringVar(&cfg.context, "context", "", "kubeconfig context")
	flags.StringVar(&cfg.output, "output", "text", "output format: text or json")
	flags.StringVar(&cfg.output, "o", "text", "output format: text or json")

	globalArgs, command, commandArgs := splitCommandArgs(args)
	if err := flags.Parse(globalArgs); err != nil {
		return commandConfig{}, "", fmt.Errorf("parse global flags: %w", err)
	}
	if command == "" {
		printUsage(stdout)
		return commandConfig{}, "", errors.New("command is required")
	}

	cfg.args = commandArgs
	if cfg.output != "text" && cfg.output != "json" {
		return commandConfig{}, "", fmt.Errorf("unsupported output format %q", cfg.output)
	}

	return cfg, command, nil
}

func splitCommandArgs(args []string) ([]string, string, []string) {
	globalArgs := make([]string, 0, len(args))
	commandArgs := make([]string, 0, len(args))
	command := ""

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if command == "" {
			if isCommand(arg) {
				command = arg
				continue
			}
			globalArgs = append(globalArgs, arg)
			continue
		}

		if isGlobalFlag(arg) {
			globalArgs = append(globalArgs, arg)
			if !strings.Contains(arg, "=") && i+1 < len(args) {
				i++
				globalArgs = append(globalArgs, args[i])
			}
			continue
		}
		commandArgs = append(commandArgs, arg)
	}

	return globalArgs, command, commandArgs
}

func isCommand(arg string) bool {
	switch arg {
	case "apply", "status", "delete", "list", "version":
		return true
	default:
		return false
	}
}

func isGlobalFlag(arg string) bool {
	name := strings.TrimLeft(arg, "-")
	if before, _, ok := strings.Cut(name, "="); ok {
		name = before
	}
	switch name {
	case "namespace", "kubeconfig", "context", "output", "o":
		return strings.HasPrefix(arg, "-")
	default:
		return false
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: chaos-ctrl [global flags] <apply|status|delete|list|version> [args]")
	fmt.Fprintln(w, "examples:")
	fmt.Fprintln(w, "  chaos-ctrl apply -f manifests/chaos-mesh/experiments/cpu-stress.yaml")
	fmt.Fprintln(w, "  chaos-ctrl status cpu-stress")
	fmt.Fprintln(w, "  chaos-ctrl delete cpu-stress")
}

func runApply(ctx context.Context, cfg commandConfig, client chaosAPI) error {
	applyFlags := flag.NewFlagSet("apply", flag.ContinueOnError)
	applyFlags.SetOutput(cfg.stderr)
	applyFlags.StringVar(&cfg.file, "f", "", "experiment YAML file")
	applyFlags.StringVar(&cfg.file, "file", "", "experiment YAML file")
	if err := applyFlags.Parse(cfg.args); err != nil {
		return fmt.Errorf("parse apply flags: %w", err)
	}
	if cfg.file == "" {
		return errors.New("apply requires -f/--file")
	}

	spec, err := experimentSpecFromFile(cfg)
	if err != nil {
		return err
	}
	if spec.Namespace == "" {
		spec.Namespace = cfg.namespace
	}

	id, err := client.Apply(ctx, spec)
	if err != nil {
		return fmt.Errorf("apply experiment: %w", err)
	}

	return writeOutput(cfg.stdout, cfg.output, map[string]string{
		"status": "applied",
		"id":     id,
	})
}

func runStatus(ctx context.Context, cfg commandConfig, client chaosAPI) error {
	if len(cfg.args) != 1 {
		return errors.New("status requires exactly one experiment name")
	}
	status, err := client.Status(ctx, cfg.args[0])
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}
	return writeOutput(cfg.stdout, cfg.output, status)
}

func runDelete(ctx context.Context, cfg commandConfig, client chaosAPI) error {
	if len(cfg.args) != 1 {
		return errors.New("delete requires exactly one experiment name")
	}
	if err := client.Delete(ctx, cfg.args[0]); err != nil {
		return fmt.Errorf("delete experiment: %w", err)
	}
	return writeOutput(cfg.stdout, cfg.output, map[string]string{
		"status": "deleted",
		"id":     cfg.args[0],
	})
}

func runList(ctx context.Context, cfg commandConfig, client chaosAPI) error {
	if len(cfg.args) != 0 {
		return errors.New("list does not accept arguments")
	}
	statuses, err := client.List(ctx)
	if err != nil {
		return fmt.Errorf("list experiments: %w", err)
	}
	return writeOutput(cfg.stdout, cfg.output, statuses)
}

func experimentSpecFromFile(cfg commandConfig) (chaos.ExperimentSpec, error) {
	contents, err := cfg.readFile(cfg.file)
	if err != nil {
		return chaos.ExperimentSpec{}, fmt.Errorf("read %s: %w", cfg.file, err)
	}

	var manifest map[string]any
	if err := yaml.Unmarshal(contents, &manifest); err != nil {
		return chaos.ExperimentSpec{}, fmt.Errorf("decode %s: %w", cfg.file, err)
	}

	kind, _ := manifest["kind"].(string)
	metadata, _ := manifest["metadata"].(map[string]any)
	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)
	spec, _ := manifest["spec"].(map[string]any)

	if name == "" {
		return chaos.ExperimentSpec{}, errors.New("manifest metadata.name is required")
	}
	if kind == "" {
		return chaos.ExperimentSpec{}, errors.New("manifest kind is required")
	}
	if spec == nil {
		return chaos.ExperimentSpec{}, errors.New("manifest spec is required")
	}

	return chaos.ExperimentSpec{
		Name:      name,
		Namespace: namespace,
		Kind:      kind,
		Spec:      spec,
	}, nil
}

func writeOutput(w io.Writer, format string, payload any) error {
	if format == "json" {
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		if _, err := fmt.Fprintf(w, "%s\n", encoded); err != nil {
			return fmt.Errorf("write json output: %w", err)
		}
		return nil
	}

	switch value := payload.(type) {
	case chaos.ExperimentStatus:
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", value.Name, value.Namespace, value.Kind, value.Phase); err != nil {
			return fmt.Errorf("write status output: %w", err)
		}
		return nil
	case []chaos.ExperimentStatus:
		for _, status := range value {
			if err := writeOutput(w, "text", status); err != nil {
				return err
			}
		}
		return nil
	case map[string]string:
		parts := make([]string, 0, len(value))
		for key, val := range value {
			parts = append(parts, fmt.Sprintf("%s=%s", key, val))
		}
		if _, err := fmt.Fprintln(w, strings.Join(parts, " ")); err != nil {
			return fmt.Errorf("write map output: %w", err)
		}
		return nil
	default:
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode output: %w", err)
		}
		if _, err := fmt.Fprintf(w, "%s\n", encoded); err != nil {
			return fmt.Errorf("write default output: %w", err)
		}
		return nil
	}
}

func newChaosClient(ctx context.Context, cfg commandConfig) (chaosAPI, error) {
	restConfig, err := kubernetesConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return chaos.NewChaosClient(client, cfg.namespace), nil
}

func kubernetesConfig(cfg commandConfig) (*rest.Config, error) {
	if cfg.kubeconfig == "" {
		if restConfig, err := rest.InClusterConfig(); err == nil {
			return restConfig, nil
		}
		cfg.kubeconfig = os.Getenv("KUBECONFIG")
	}
	if cfg.kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve home directory: %w", err)
		}
		cfg.kubeconfig = filepath.Join(home, ".kube", "config")
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: cfg.kubeconfig}
	overrides := &clientcmd.ConfigOverrides{}
	if cfg.context != "" {
		overrides.CurrentContext = cfg.context
	}
	restConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	return restConfig, nil
}
