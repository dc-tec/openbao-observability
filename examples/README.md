# Examples

This directory contains runnable deployment examples for local, Kubernetes, and
VM-style OpenBao observability profiles.

- `docker-compose/`: local OpenBao, Prometheus, Loki, Alloy, and Grafana stack.
- `audit-archive-health/`: example exporter and recording-rule mapping for the
  audit archive health metrics.
- `kubernetes/`: reusable Kubernetes manifests for secure active-node and
  private all-node scrape profiles.
- `kubernetes/audit-archive-health-scrape.yaml`: Service and ServiceMonitor
  example for scraping an audit archive health exporter.
- `kubernetes/kind/operator-managed/`: kind validation profile for generated
  observability artifacts against an operator-managed OpenBao cluster.
- `kubernetes/operator-managed/`: merge patches and adoption steps for
  OpenBao clusters managed by `dc-tec/openbao-operator`.
- `synthetic-probes/`: optional blackbox-style probe contract for SLO and
  availability dashboard panels and alerts.

## Maturity labels

| Example family | Lifecycle | Evidence |
| -------------- | --------- | -------- |
| `docker-compose/` | `stable` for local evaluation | `profile-validated` when live profile checks pass |
| `audit-archive-health/` | `reference` | `documented` |
| `kubernetes/` | `reference` | `documented` |
| `kubernetes/kind/operator-managed/` | `reference` | `profile-validated` when kind validation passes |
| `kubernetes/operator-managed/` | `reference` | `documented` |
| `synthetic-probes/` | `reference` | `documented` |
