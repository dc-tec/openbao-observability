# Project status and maturity

Use this reference to understand what the OpenBao Observability repository
validates today and what you still need to validate in your own environment. It
is for operators and platform teams who need to decide how much of the
reference architecture to adopt.

This repository is a validated reference architecture with an implemented local
profile. It is not a drop-in production monitoring distribution. Use the
generated artifacts as a starting point, then apply your own retention, access
control, routing, packaging, and release process.

## Maturity matrix

| Area | Current state | Confidence | You still verify |
| ---- | ------------- | ---------- | ---------------- |
| Metrics contracts | Generated from source contracts and validated against OpenBao `2.5.5` fixtures. | High | Your OpenBao version, metric prefix, scrape labels, and topology. |
| Metric prefix variants | Generated for `vault_*` and `openbao_*` source metrics. | High | The prefix that your OpenBao telemetry configuration emits. |
| Prometheus recording rules | Generated for native Prometheus and Prometheus Operator manifests. | High | Rule load, evaluation cost, retention, and label policy in your metrics backend. |
| Prometheus alert rules | Generated from alert contracts with runbook links. | High | Alert routing, paging policy, inhibition, and environment-specific thresholds. |
| Loki alert references | Generated reference artifacts for log and audit detections. | Medium | Your Loki ruler setup, log volume, retention, and alert routing. |
| Grafana dashboards | Generated from dashboard contracts and validated for syntax and query structure. | Medium to high | Data-source UIDs, live query results, dashboard permissions, and production label shape. |
| Docker Compose profile | Implemented local profile with OpenBao HA, PostgreSQL, Prometheus, Loki, Alloy, Grafana, and fixture scenarios. | High for local evaluation | Production hardening, TLS, credentials, network isolation, storage, and secrets. |
| Kubernetes scrape examples | Implemented examples for secure active-node and private all-node scraping. | Medium | Service discovery labels, NetworkPolicy, TLS, RBAC, and Prometheus Operator conventions. |
| OpenBao Operator companion profile | Implemented companion profile and integration contract. | Medium | Compatibility with the operator release and your `OpenBaoCluster` configuration. |
| Audit archive design | Reference design plus local archive health profile. | Early to medium | Compliance requirements, immutability, encryption, retention, evidence export, and access review. |
| Security audit detections | Reference detections and dashboards based on audit log patterns. | Medium | Local threat model, expected activity, false positives, and escalation policy. |
| Feature-specific dashboards | Implemented for auth, identity, tokens, leases, database secrets, Transit, PKI, mounts, runtime, storage, namespaces, Kubernetes, and SLOs. | Medium | Feature enablement, namespaces, auth methods, secret engines, and data availability in your deployment. |

## How to consume the repository

Choose the path that matches the way you operate OpenBao.

| You use | Start with | Consume |
| ------- | ---------- | ------- |
| Prometheus and Grafana | [Prometheus, Loki, Grafana, and Alloy profile](../implementation-profiles/prometheus-loki-grafana-alloy.md) | `generated/prometheus/`, `generated/prometheusrules/`, `generated/loki/`, and `generated/grafana/`. |
| Kubernetes with Prometheus Operator | [Secure metrics scrape](../metrics/secure-metrics-scrape.md) and [all-node metrics scrape](../metrics/all-node-metrics-scrape.md) | PrometheusRule manifests, ServiceMonitor examples, and Kubernetes examples. |
| The OpenBao Operator | [OpenBao Operator companion profile](../implementation-profiles/openbao-operator.md) | Operator-managed examples and the integration contract. |
| Another observability backend | [Adopt the reference architecture](./adoption.md) | Signal contracts, alert intent, dashboard questions, and runbook structure. |
| Local evaluation | [Run the Docker Compose stack](../docker-compose.md) | The local HA profile, generated dashboards, and fixture scenarios. |
| Repository contribution | [Contributing](../../CONTRIBUTING.md) | Contracts, generators, fixtures, validation, and generated artifacts. |

## What the repository proves

The validation suite proves that the source contracts, generated artifacts, and
local fixture profile stay internally consistent. It checks generated output,
dashboard contracts, stream contracts, fixture expectations, documentation
links, Prometheus rule syntax, Go tests, linting, and vulnerability checks.

This evidence helps you trust the reference architecture. It does not prove
that your production deployment has safe retention, correct paging, complete
archive controls, or enough network isolation.

## What remains your responsibility

You own these decisions in production:

- Metrics, log, and audit retention windows.
- Dashboard, query, and audit-log access control.
- Alert routing, paging, inhibition, and escalation.
- SIEM, compliance archive, evidence export, and legal hold.
- TLS, mTLS, NetworkPolicy, firewall rules, and collector credentials.
- Resource sizing for Prometheus, Loki, Grafana, Alloy, and downstream
  platforms.
- Promotion of generated artifacts into GitOps, Helm, Terraform, or another
  delivery pipeline.
- Version compatibility testing for your OpenBao, OpenBao Operator, and
  observability platform releases.

## Non-goals

This repository does not try to be every part of a production observability
platform.

- It is not a turnkey production monitoring distribution.
- It is not a compliance archive or SIEM product.
- It is not a replacement for the OpenBao Operator.
- It is not a complete Helm chart for every platform convention.
- It does not define your paging policy, access model, retention policy, or
  audit evidence process.
- It does not guarantee every dashboard panel is populated in every OpenBao
  topology.
- It does not make Loki an authoritative long-term audit archive by itself.
- It does not cover tracing or application instrumentation beyond the OpenBao
  signal model.

## Roadmap

The roadmap focuses on making the reference architecture easier to adopt while
keeping production responsibilities explicit.

### Adoption and packaging

- Make generated artifacts easier to consume from GitOps workflows.
- Keep Kubernetes examples practical, without becoming a full platform
  installer.
- Keep OpenBao Operator semantics in the operator repository and maintain this
  repository as the observability companion.

### Compatibility and validation

- Track OpenBao version compatibility, starting with OpenBao `2.5.5`.
- Add compatibility notes for relevant OpenBao Operator releases.
- Expand fixture coverage for namespaces, read replicas, HA/Raft, and
  feature-specific secret engines.

### Dashboard and alert depth

- Continue feature-specific dashboards where OpenBao exposes useful, safe
  signals.
- Add alerts when thresholds are defensible from fixtures, documentation, or
  operational reasoning.
- Keep suspicious-activity detections as reference security signals, not a SIEM
  replacement.

### Documentation maturity

- Add a dashboard tour with screenshots or a compact guided walkthrough.
- Keep concept docs focused on how OpenBao observability works.
- Keep production responsibilities explicit: retention, access control, paging,
  and audit archive remain user-owned.

## What's next

- Use [Reference architecture overview](./overview.md) to understand the
  architecture boundary.
- Use [Adopt the reference architecture](./adoption.md) to map this design to
  your platform.
- Use [Implementation profiles](../implementation-profiles/README.md) to choose
  the closest starting profile.
- Use [Prometheus, Loki, Grafana, and Alloy profile](../implementation-profiles/prometheus-loki-grafana-alloy.md)
  when you want to consume the generated artifacts directly.
- Use [OpenBao Operator companion profile](../implementation-profiles/openbao-operator.md)
  when the OpenBao Operator manages your clusters.
