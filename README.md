# OpenBao observability reference architecture

This repository builds a reference architecture for observing OpenBao with
metrics, logs, audit logs, dashboards, alerts, runbooks, and validation
fixtures. Use it when you need a tested starting point for operating OpenBao
with Prometheus-compatible metrics, Grafana, Loki, and Grafana Alloy.

The project starts from verified OpenBao behavior instead of copied Vault
dashboard assumptions. Research notes live in `workstreams/`; operator-facing
documentation, contracts, examples, and generated artifacts live outside that
directory.

## Start here

- Run `make fixtures-openbao` to capture OpenBao `2.5.4` Docker fixtures.
- Run `make compose-up` to start the local Docker Compose reference stack.
- Run `make test-unit` to run Go tests without fixture capture.
- Run `make test-fixtures` to validate the captured metrics and audit samples.
- Run `make contracts-verify` to validate metric contracts against fixtures.
- Run `make generate` to generate Prometheus rules, alert artifacts, and
  Grafana dashboards.
- Run `make validate-generated` to validate generated Prometheus rule files
  with `promtool`.

Implementation planning notes are local-only files under `workstreams/` with a
`.local.md` suffix. Git ignores those files.

## Repository layout

| Path | Purpose |
| ---- | ------- |
| `contracts/` | Source contracts for metrics, log streams, alerts, and dashboards. |
| `cmd/` | Go command-line entry points for project tooling. |
| `docs/` | User-facing documentation written with the project style guide. |
| `examples/` | Runnable local and deployment examples, including Docker Compose. |
| `fixtures/` | Captured metrics and log fixtures used by tests. |
| `generated/` | Generated artifacts produced from contracts. |
| `internal/` | Go packages that implement fixture capture and validation. |
| `tests/` | Validation checks for fixtures, contracts, generated artifacts, and docs. |
| `workstreams/` | Research input, style guides, and ignored local planning notes. |
