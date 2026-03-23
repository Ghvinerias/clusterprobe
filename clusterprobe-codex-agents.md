# ClusterProbe — Codex Agent Definitions & Prompts

> Synthetic load generator and cluster validation harness for on-premise Kubernetes.
> One namespace. Fully self-contained. Deploy, hammer, observe, chaos, teardown.

---

## Repository Layout (target)

```
clusterprobe/
├── cmd/
│   ├── api/            # Go REST API + WebSocket server
│   ├── worker/         # Go RabbitMQ consumer pool
│   ├── ui/             # Go HTMX UI server
│   └── chaos-ctrl/     # chaos trigger sidecar
├── internal/
│   ├── config/         # shared config parsing
│   ├── db/             # Postgres, Redis, MongoDB wrappers
│   ├── messaging/      # RabbitMQ producer/consumer abstractions
│   ├── telemetry/      # OTEL bootstrap (tracer, meter, logger)
│   ├── workload/       # synthetic workload generators
│   └── chaos/          # Chaos Mesh CRD client wrappers
├── web/
│   ├── templates/      # Go HTML templates (HTMX)
│   └── static/         # CSS, minimal JS
├── deploy/
│   ├── kustomize/
│   │   ├── base/
│   │   └── overlays/
│   │       ├── self-contained/   # bundles full Grafana stack
│   │       └── existing-stack/   # reuses cluster observability
│   └── helm/
│       └── clusterprobe/
├── manifests/
│   ├── chaos-mesh/     # Chaos Mesh install + experiment CRs
│   └── observability/  # Alloy, Prometheus, Loki, Tempo, Grafana
├── dashboards/         # Grafana dashboard JSON (mounted as ConfigMaps)
├── scripts/
│   └── review/         # lint, vet, staticcheck, gosec, coverage runner
└── docs/
```

---

## Global Agent Conventions

These rules apply to **every agent** in every phase. Each agent must read and
internalize this section before doing any work.

### Language & Style
- Go 1.22+
- Module: `github.com/Ghvinerias/clusterprobe`
- `gofmt` + `goimports` enforced on every file before commit
- Errors wrapped: `fmt.Errorf("context: %w", err)` — never discarded silently
- No `panic()` in library or service code
- `context.Context` is the first argument on every function that does I/O
- All exported functions and types carry godoc comments

### Logging
- `log/slog` (stdlib), JSON handler, written to stdout
- Every log entry carries structured fields: `service`, `phase`, `trace_id`
- Log levels: DEBUG for internal state, INFO for lifecycle events, WARN for
  recoverable errors, ERROR for failures that affect correctness
- Log format must be ingestible by Loki without transformation

### Observability (OTEL)
- Bootstrap in `internal/telemetry/otel.go`, called as `telemetry.Init(ctx, cfg)`
- Traces: `go.opentelemetry.io/otel` with OTLP/gRPC exporter → Alloy
- Metrics: `go.opentelemetry.io/otel/metric` with OTLP/gRPC exporter → Alloy
- Every HTTP handler and RabbitMQ consumer must create a child span
- Propagate trace context across service boundaries (W3C TraceContext)

### Testing
- Unit tests alongside source (`_test.go`)
- Integration tests under `/integration` with build tag `//go:build integration`
- Minimum 70% coverage on `internal/` packages
- Tests must pass `go test ./...` with no flags before any commit

### Docker
- Multi-stage `Dockerfile` per `cmd/`
- Builder stage: `golang:1.22-alpine`
- Final stage: `gcr.io/distroless/static-debian12`
- No root user (`USER 65534:65534`)
- Build args: `VERSION`, `COMMIT_SHA`, `BUILD_DATE` baked into binary via ldflags

### Commit Discipline
This is critical. Agents must not batch all work into one commit.

**Rules:**
1. Commit after each logical unit of work — a file group, a feature, a config
   block. A logical unit is: "this compiles and makes sense on its own."
2. Commit message format (Conventional Commits):
   ```
   <type>(<scope>): <short imperative description>

   [optional body — what and why, not how]
   [optional: Refs #issue]
   ```
   Types: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `ci`, `build`
   Scopes match directory or component: `api`, `worker`, `ui`, `db`, `telemetry`,
   `chaos`, `deploy`, `helm`, `kustomize`, `alloy`, `scaffold`

3. Examples of good commits:
   ```
   chore(scaffold): init go module and workspace layout
   feat(telemetry): add OTEL bootstrap with OTLP/gRPC exporter
   feat(db): add Postgres client wrapper with connection pool
   test(db): add unit tests for Postgres retry logic
   fix(worker): handle RabbitMQ reconnect on channel close
   docs(api): add godoc for LoadProfile and ScenarioRequest types
   build(docker): add multi-stage Dockerfile for api service
   chore(kustomize): add base namespace and commonLabels
   ```

4. Never commit:
   - Code that does not compile
   - Files with `gofmt` violations
   - Hardcoded secrets or credentials (use env vars or k8s Secrets)
   - Commented-out dead code blocks

5. After every 3–5 commits, run `go build ./...` and `go test ./...` and commit
   any resulting fixes before continuing.
