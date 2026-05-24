# Documentation

This directory contains user-facing documentation for the OpenBao Observability
reference architecture. Write each page as a how-to, runbook, reference, or
explainer.

Start with [Run the Docker Compose stack](./docker-compose.md) when you want a
local OpenBao, Prometheus, Loki, Alloy, and Grafana environment.

## Operate

- [Run the Docker Compose stack](./docker-compose.md)
- [Configure a secure metrics scrape](./metrics/secure-metrics-scrape.md)
- [Configure an all-node metrics scrape](./metrics/all-node-metrics-scrape.md)
- [Configure declarative audit devices](./audit/declarative-audit.md)

## Understand

- [OpenBao observability model](./concepts/openbao-observability-model.md)
- [Metrics, logs, and audit logs](./concepts/metrics-vs-logs-vs-audit-logs.md)
- [High-cardinality and label safety](./concepts/high-cardinality-and-label-safety.md)

## Dashboards

- [OpenBao overview dashboard](./dashboards/overview-dashboard.md)
- [OpenBao HA/Raft dashboard](./dashboards/ha-raft.md)
- [OpenBao audit overview dashboard](./dashboards/audit-overview.md)
- [OpenBao audit investigation dashboard](./dashboards/audit-investigation.md)
- [OpenBao operational logs dashboard](./dashboards/operational-logs.md)
- [OpenBao auth and identity dashboard](./dashboards/auth-identity.md)
- [OpenBao token and lease lifecycle dashboard](./dashboards/token-lease-lifecycle.md)
- [OpenBao secret engines and mounts dashboard](./dashboards/secret-engines-mounts.md)

## Reference

- [Understand metric prefixes and recording rules](./contracts/metric-prefix.md)

## Respond

Use these runbooks when the generated alert rules fire:

- [OpenBao metrics scrape failing](./runbooks/openbao-metrics-scrape-failing.md)
- [OpenBao sealed unexpectedly](./runbooks/openbao-sealed-unexpectedly.md)
- [No active OpenBao leader](./runbooks/no-active-openbao-leader.md)
- [Multiple active OpenBao nodes](./runbooks/multiple-active-nodes.md)
- [OpenBao Raft and Autopilot health](./runbooks/raft-autopilot-health.md)
- [Audit request and response failures](./runbooks/audit-request-response-failures.md)
- [Audit canary missing](./runbooks/audit-canary-missing.md)
- [Audit log stream missing](./runbooks/audit-log-stream-missing.md)
- [Operational log stream missing](./runbooks/operational-log-stream-missing.md)
- [Debug logging enabled](./runbooks/debug-logging-enabled.md)
- [Irrevocable leases present](./runbooks/irrevocable-leases.md)

Do not put implementation plans, work notes, or contributor-only design notes
in this directory. Put those files under `workstreams/` with a `.local.md`
suffix.
