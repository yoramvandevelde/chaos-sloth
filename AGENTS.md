# AGENTS.md

Small Go CLI that runs chaos engineering against Proxmox VMs. Single module, no sub-packages beyond `internal/`.

## Build & run

```bash
make build          # → ./chaos-sloth binary
make run            # go run . -config config.yaml (needs real Proxmox creds)
make tidy           # go mod tidy
go test ./...       # run all tests
go test -race ./... # run with race detector
```

No linter, formatter, or typecheck are defined in this repo. CI (`.github/workflows/build.yml`) only builds and pushes the Docker image to `ghcr.io` on push to `main`.

## Project structure

- `main.go` — entrypoint, wires config + proxmox client + chaos loop
- `internal/config/config.go` — YAML + env-var config loading with validation; env vars always override file
- `internal/config/config_test.go` — tests for config loading, env overrides, validation, YAML expansion
- `internal/config/testdata/` — test fixtures (valid.yaml, invalid.yaml, missing_fields.yaml)
- `internal/proxmox/client.go` — thin HTTP client for the Proxmox API (token auth, POST-only)
- `internal/proxmox/client_test.go` — tests using `httptest.NewServer` as fake Proxmox API
- `internal/chaos/sloth.go` — main loop: pick random target, strike, auto-recover; uses `VMClient` interface for testability
- `internal/chaos/sloth_test.go` — tests with `fakeClient` mock: all actions, jitter, dry-run, shutdown recovery
- `chart/` — Helm chart for Kubernetes deployment (single replica, env-var driven)
- `Dockerfile` — multi-stage Go build, Alpine runtime

## Key conventions

- **Tests exist.** Run `go test ./...` after changes. Tests use `httptest.NewServer` (proxmox) and a `fakeClient` mock (chaos) — no real Proxmox needed.
- **Env vars beat config file.** The config loader applies env overrides after parsing YAML; `CHAOS_TARGETS` expects a JSON array.
- **`resume_after` defaults to 5m** for hibernate/pause if not set or zero — this is hardcoded in validation, not just a doc default.
- **`random` action excludes `random` itself** — only hibernate, pause, stop, reset are candidates.
- **Graceful shutdown** — on SIGINT/SIGTERM, the chaos loop will attempt to recover any currently disrupted VM before exiting (10s timeout).
- **Helm chart: always single replica** — `replicas: 1` is intentional; multiple would fire duplicate chaos events.
- **Config file supports `${ENV_VAR}` expansion** via `os.ExpandEnv` before YAML parsing (`internal/config/config.go:72`).
