# Implementation profiles

Use this reference to choose an implementation profile for the OpenBao
Observability reference architecture. It is for operators who need to decide
which parts of the included implementation to run, port, or replace.

## What a profile defines

An implementation profile maps the portable reference architecture to concrete
tools and delivery mechanisms.

| Area | Profile decision |
| ---- | ---------------- |
| Metrics collection | Scrape target, authentication, TLS, labels, and service discovery. |
| Metrics backend | Storage, query language, recording rules, alert evaluation, and retention. |
| Log collection | Collector, source paths, stream labels, parsing, and delivery credentials. |
| Audit handling | Restricted exploration backend, archive path, retention, and access model. |
| Dashboards | Dashboard format, folders, data sources, permissions, and variables. |
| Alerts | Rule format, routing labels, paging integration, and runbook links. |
| Validation | Fixture capture, syntax checks, live query checks, and platform checks. |

## Available profiles

| Profile | Status | Use it for | Main entrypoint |
| ------- | ------ | ---------- | --------------- |
| Prometheus, Loki, Grafana, and Alloy | Implemented | Generated artifacts, local validation, and direct adoption by teams already using this stack. | [Prometheus, Loki, Grafana, and Alloy profile](./prometheus-loki-grafana-alloy.md) |
| Docker Compose | Implemented local profile | Evaluation, screenshots, dashboard review, fixture scenarios, and live query validation. | [Run the Docker Compose stack](../docker-compose.md) |
| Kubernetes secure active-node scrape | Implemented example | Production-oriented authenticated metrics collection from the active OpenBao node. | [Secure metrics scrape](../metrics/secure-metrics-scrape.md) |
| Kubernetes private all-node scrape | Implemented example | Per-node HA, Raft, standby, sealed-node, and runtime visibility. | [All-node metrics scrape](../metrics/all-node-metrics-scrape.md) |
| OpenBao Operator companion | Companion profile | Applying this reference architecture to OpenBao clusters managed by `dc-tec/openbao-operator`. | [OpenBao Operator companion profile](./openbao-operator.md) |
| OpenBao Operator integration contract | Reference | Resource, label, scrape, dashboard, alert, and log boundaries for operator-managed OpenBao clusters. | [OpenBao Operator integration contract](./openbao-operator-integration-contract.md) |
| Bring your own backend | Adoption pattern | Mapping the same architecture to another metrics, logging, dashboard, alerting, or SIEM platform. | [Adopt the reference architecture](../reference-architecture/adoption.md) |

## Profile boundaries

Profiles can change the tools, not the signal semantics.

A profile can replace Prometheus with another metrics backend, Loki with
another log backend, Grafana with another dashboard layer, or Alloy with
another collector. It still needs to preserve the OpenBao signal model,
low-cardinality label policy, audit-log access boundary, alert intent, and
runbook linkage.

## Choose a profile

Use the Docker Compose profile when you want to see the architecture running on
one workstation. Use it to review dashboards and validate queries against live
OpenBao behavior.

Use the Prometheus, Loki, Grafana, and Alloy profile when your platform already
uses that stack or when you want generated artifacts that can move into GitOps,
Helm, Terraform, or another delivery system.

Use the secure active-node scrape as the metrics baseline when you need the
lowest-risk production starting point.

Use the private all-node scrape when you need standby, sealed-node, follower,
or per-node Raft visibility and have approved network isolation for the metrics
listener.

Use the OpenBao Operator companion profile when the operator owns Kubernetes
lifecycle, tenancy, TLS, unseal, backups, restores, upgrades, and read scaling,
and this repository owns the OpenBao workload observability contract.

Use the OpenBao Operator integration contract when you need the concrete labels,
Services, scrape profiles, log streams, dashboard ownership, and alert ownership
that connect the two repositories.

Use the bring-your-own-backend pattern when your organization already has a
standard observability platform. Port the architecture, not the local demo
stack.

## What's next

- Use [Reference architecture overview](../reference-architecture/overview.md)
  to understand what stays portable across profiles.
- Use [Adopt the reference architecture](../reference-architecture/adoption.md)
  to map the architecture to your environment.
- Use [Prometheus, Loki, Grafana, and Alloy profile](./prometheus-loki-grafana-alloy.md)
  to inspect the implemented profile.
- Use [OpenBao Operator companion profile](./openbao-operator.md) when
  `dc-tec/openbao-operator` manages the OpenBao clusters.
- Use [OpenBao Operator integration contract](./openbao-operator-integration-contract.md)
  when you need to align operator-managed resources with this repository's
  generated artifacts.
