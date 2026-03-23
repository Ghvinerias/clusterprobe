# AGENTS.md

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

## Phase Index

| Phase | Deliverable | Agent |
| --- | --- | --- |
| 1 | Go module, workspace, and gitignore | scaffold-agent |
| 2 | Repository directory skeleton and README | scaffold-agent |
| 3 | Linting/review tooling | scaffold-agent |
| 4 | Telemetry bootstrap stub | scaffold-agent |
| 5 | Shared configuration loader | scaffold-agent |
| 6 | Kustomize base namespace | scaffold-agent |
| 7 | Helm chart skeleton | scaffold-agent |
| 8 | AGENTS.md source of truth | scaffold-agent |
