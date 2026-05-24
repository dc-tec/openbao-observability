# OpenBao observability reference architecture

[![CI](https://github.com/dc-tec/openbao-observability/actions/workflows/ci.yml/badge.svg)](https://github.com/dc-tec/openbao-observability/actions/workflows/ci.yml)

Use this repository as an OpenBao observability reference architecture for
metrics, operational logs, audit logs, dashboards, alerts, runbooks, and
validation fixtures. It defines portable observability intent first, then
provides a tested Prometheus, Loki, Grafana, and Grafana Alloy implementation
profile that you can adapt to your monitoring and logging platforms.

The project starts from verified OpenBao behavior instead of copied Vault
dashboard assumptions. Contracts under `contracts/` describe the source signal
model; generated artifacts under `generated/` show one concrete implementation
profile.

## What you get

- Signal contracts for OpenBao metrics, log streams, alerts, and dashboards.
- Generated Prometheus recording rules and alert rules.
- Generated Loki alert reference artifacts.
- Generated Grafana dashboard JSON files.
- Grafana Alloy examples for operational logs, audit logs, and collection
  pipelines.
- Runnable Docker Compose and Kubernetes examples.
- Fixture capture and validation for verified OpenBao behavior.
- Operator documentation for metrics, logs, audit logs, dashboards, alerts, and
  runbooks.

## Architecture at a glance

Every implementation profile maps the same OpenBao signals to a local
monitoring stack. The included profile uses Prometheus for metrics, Loki for
logs and audit logs, Grafana Alloy for collection, and Grafana for dashboards.

```text
OpenBao nodes
  | metrics
  v
Metrics backend -> alert rules -> runbooks
  |
  v
Dashboards

OpenBao operational logs -> collector -> log backend -> alerts
OpenBao audit logs       -> collector -> log backend -> security runbooks
```

## Use this with your platforms

Adopt the architecture by preserving the OpenBao signal semantics and mapping
the storage, query, alerting, and dashboard layers to your environment.

- Port metric contracts and alert intent to your metrics backend.
- Port log and audit log detections to your log analytics backend.
- Keep label and attribute choices low-cardinality and safe for shared systems.
- Treat audit logs as protected security records with explicit retention and
  access controls.
- Treat dashboard panels as operator questions, then implement those questions
  in your visualization layer.
- Keep runbooks close to the alerts that page your team.

## Tested implementation profile

The current implementation profile includes:

- OpenBao `2.5.4` fixture capture.
- Prometheus-compatible OpenBao metrics.
- Prometheus recording rules and alert rules.
- Loki log and audit log alert reference artifacts.
- Grafana dashboards generated from dashboard contracts.
- Grafana Alloy collection examples.
- A local Docker Compose stack with OpenBao, PostgreSQL, Prometheus, Loki,
  Grafana Alloy, and Grafana.
- Kubernetes examples for secure active-node and private all-node metrics
  scrape profiles.

> [!WARNING]
> The Docker Compose stack is for local evaluation and contract validation. It
> uses HTTP, deterministic local credentials, and local-only OpenBao setup. You
> must not use it for production, shared environments, or sensitive data.

## Quick start

Run these commands from the repository root.

```shell
make fixtures-openbao
make generate
make compose-up
```

Open Grafana at `http://127.0.0.1:13000` and sign in with `admin` / `admin`.
See [Run the Docker Compose stack](docs/docker-compose.md) for the complete
local setup, verification steps, and local endpoints.

Stop the local stack when you finish.

```shell
make compose-down
```

## Documentation

Start with the documentation index when you want the full operator-facing
documentation set.

- [Documentation index](docs/README.md)
- [Reference architecture overview](docs/reference-architecture/overview.md)
- [Adopt the reference architecture](docs/reference-architecture/adoption.md)
- [Implementation profiles](docs/implementation-profiles/README.md)
- [Prometheus, Loki, Grafana, and Alloy][prometheus-loki-grafana-alloy]
- [OpenBao Operator companion profile](docs/implementation-profiles/openbao-operator.md)
- [OpenBao Operator integration contract](docs/implementation-profiles/openbao-operator-integration-contract.md)
- [OpenBao observability model](docs/concepts/openbao-observability-model.md)
- [Metrics, logs, and audit logs](docs/concepts/metrics-vs-logs-vs-audit-logs.md)
- [High-cardinality and label safety](docs/concepts/high-cardinality-and-label-safety.md)
- [Audit logs as security records](docs/concepts/audit-logs-as-security-records.md)
- [Run the Docker Compose stack](docs/docker-compose.md)
- [Secure metrics scrape](docs/metrics/secure-metrics-scrape.md)
- [All-node metrics scrape](docs/metrics/all-node-metrics-scrape.md)
- [Dashboard documentation](docs/README.md#dashboards)
- [Alert runbooks](docs/README.md#respond)

## Contracts and generated artifacts

Edit the source contracts under `contracts/`, then regenerate artifacts.

```shell
make generate
```

Generated artifacts live under `generated/` and are not edited by hand:

- `generated/prometheus/`: native Prometheus rule files.
- `generated/prometheusrules/`: Prometheus Operator `PrometheusRule` manifests.
- `generated/loki/`: Loki alert reference artifacts.
- `generated/grafana/`: Grafana dashboard JSON files.
- `generated/docs/`: generated reference documents.

Use generated artifacts as versioned release assets or as inputs to your own
platform delivery pipeline.

## Validate changes

Run focused validation while you work.

```shell
make test-unit
make test-fixtures
make contracts-verify
make docs-verify
make validate-generated
```

Validate dashboard PromQL and LogQL against a running Compose stack.

```shell
make validate-dashboard-queries
```

Run the full generated-artifact verification before you publish changes.

```shell
make verify
```

## Contributing and license

Use [Contributing](CONTRIBUTING.md) before you change docs, contracts,
examples, generated artifacts, or validation code.

This project is licensed under the [Apache License, Version 2.0](LICENSE).
Apache-2.0 is a permissive open source license with an explicit patent grant,
which fits a reusable reference architecture that teams can adapt to their own
environments.

Copyright 2026 OpenBao Observability contributors.

## Repository layout

| Path | Purpose |
| ---- | ------- |
| `alerts/` | Generated and hand-reviewed alert artifacts. |
| `alloy/` | Grafana Alloy examples for OpenBao collection pipelines. |
| `cmd/` | Go command-line entry points for project tooling. |
| `contracts/` | Source contracts for metrics, log streams, alerts, and dashboards. |
| `dashboards/` | Dashboard documentation and dashboard-specific source material. |
| `docs/` | User-facing documentation written with the project style guide. |
| `examples/` | Runnable local and deployment examples, including Docker Compose. |
| `fixtures/` | Captured metrics and log fixtures used by tests. |
| `generated/` | Generated artifacts produced from contracts. |
| `internal/` | Go packages that implement fixture capture and validation. |
| `tests/` | Validation checks for fixtures, contracts, generated artifacts, and docs. |
| `workstreams/` | Research input, style guides, and ignored local planning notes. |

Implementation planning notes are local-only files under `workstreams/` with a
`.local.md` suffix. Git ignores those files.

[prometheus-loki-grafana-alloy]: docs/implementation-profiles/prometheus-loki-grafana-alloy.md
