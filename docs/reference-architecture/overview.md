# Reference architecture overview

Use this explainer to understand the OpenBao Observability reference
architecture before you choose a monitoring stack. It is for operators who need
portable OpenBao observability guidance that can be implemented with different
metrics, logging, dashboard, and alerting platforms.

## Architecture boundary

The reference architecture defines the OpenBao signals, derived signals, access
boundaries, and operational questions that an observability deployment needs to
answer. It does not require Prometheus, Loki, Grafana, or Grafana Alloy.

The repository includes a tested Prometheus, Loki, Grafana, and Grafana Alloy
implementation profile because those tools provide a concrete way to validate
the architecture. Treat that profile as a working implementation, not as the
architecture boundary.

## Signal layers

| Layer | What you define | Repository starting point |
| ----- | --------------- | ------------------------- |
| Source signals | OpenBao metrics, operational logs, completed request logs, audit logs, and platform state. | [OpenBao observability model](../concepts/openbao-observability-model.md) |
| Collection | Scrape targets, collectors, authentication, network controls, and log routing. | [Secure metrics scrape](../metrics/secure-metrics-scrape.md), [all-node metrics scrape](../metrics/all-node-metrics-scrape.md), and the [Docker Compose Alloy config](../../examples/docker-compose/alloy/config.alloy) |
| Storage and query | Metrics backend, log backend, audit archive, query language, retention, and access control. | [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md), [Log retention and access control](../logging/retention-and-access-control.md), and [Audit archive reference design](../audit/audit-archive-reference-design.md) |
| Derived signals | Recording rules, alert rules, dashboard queries, and log detections. | [Metric contracts](../../contracts/metrics/openbao-core.yaml), [alert contracts](../../contracts/alerts/critical.yaml), and [dashboard contracts](../../contracts/dashboards/openbao-overview.yaml) |
| Response | Runbooks, escalation paths, ownership, and verification steps. | [Alert runbooks](../README.md#respond) |

## Logical flow

Read the architecture as a set of independent signal paths that meet again in
dashboards, alerts, and runbooks.

```text
OpenBao metrics
  -> metrics collection
  -> metrics backend
  -> recording rules
  -> dashboards, alerts, and runbooks

OpenBao operational logs
  -> log collection
  -> operational log backend
  -> operational dashboards, alerts, and runbooks

OpenBao audit logs
  -> restricted collection
  -> restricted exploration backend
  -> security archive
  -> security dashboards, detections, and runbooks

Platform signals
  -> platform collectors
  -> metrics and log backends
  -> context for OpenBao incidents
```

## Portable decisions

Keep these decisions stable when you adapt the reference architecture to another
platform:

- Preserve the distinction between metrics, operational logs, audit logs, and
  platform signals.
- Keep audit logs separate from broad operational logs at collection, storage,
  dashboard, and access-control boundaries.
- Normalize raw OpenBao metrics into stable derived signals before you depend
  on them in dashboards and alerts.
- Keep labels low-cardinality and avoid labels that expose request paths,
  secret paths, token accessors, entity identifiers, policies, or client
  addresses.
- Treat dashboard panels as operator questions. Reimplement those questions in
  your visualization layer when you do not use Grafana.
- Keep alert runbooks close to the alert definitions, even when your paging
  system owns the final notification policy.

## Adoption responsibilities

The repository gives you source contracts, examples, generated artifacts, and
validation tests. Your production environment still owns the platform-specific
choices.

| Decision | You define |
| -------- | ---------- |
| Metrics backend | Prometheus, Mimir, Thanos, VictoriaMetrics, Datadog, CloudWatch, Azure Monitor, or another system. |
| Log backend | Loki, OpenSearch, Elastic, Splunk, a SIEM, or another log analytics system. |
| Audit archive | Immutable or compliance-approved storage outside the short-term exploration path. |
| Access model | Who can read operational logs, audit logs, dashboards, alerts, and raw queries. |
| Retention | Retention windows for operational logs, audit exploration, audit archive, and metrics. |
| Scrape profile | Authenticated active-node scraping, private all-node scraping, or a hybrid profile. |
| Label policy | Bounded routing labels and forbidden high-cardinality or sensitive labels. |
| Release process | How generated artifacts enter your GitOps, Terraform, Helm, or platform pipeline. |

## What this repository validates

The current repository validates one concrete implementation profile:

- OpenBao `2.5.5` fixture capture.
- `vault_*` and `openbao_*` metric prefix variants.
- Generated Prometheus recording rules and alert rules.
- Generated Loki alert reference artifacts.
- Generated Grafana dashboards.
- Docker Compose integration with OpenBao, PostgreSQL, Prometheus, Loki,
  Grafana Alloy, and Grafana.
- Kubernetes scrape examples for secure active-node and private all-node
  metrics collection.

This validation gives you evidence that the contracts and generated artifacts
match observed OpenBao behavior. It does not prove that your production
retention, access control, alert routing, archive, or network design is
complete.

## What's next

- Use [Project status and maturity](./project-status.md) to understand what the
  repository validates today and what remains your production responsibility.
- Use [Adopt the reference architecture](./adoption.md) to map this design to
  your environment.
- Use [Implementation profiles](../implementation-profiles/README.md) to choose
  the concrete profile that matches your starting point.
- Use [Prometheus, Loki, Grafana, and Alloy profile](../implementation-profiles/prometheus-loki-grafana-alloy.md)
  when you want to use the included generated artifacts directly.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you change labels, alert groupings, or dashboard variables.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  before you expand audit-log access.
- Use [Audit archive reference design](../audit/audit-archive-reference-design.md)
  before you choose a durable audit evidence path.
