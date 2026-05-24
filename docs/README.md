# Documentation

This directory contains user-facing documentation for the OpenBao Observability
reference architecture. Use it to learn the architecture, run the local
profile, adopt the generated artifacts, read dashboards, and respond to alerts.

Start with [Run the Docker Compose stack](./docker-compose.md) when you want a
local OpenBao, Prometheus, Loki, Alloy, and Grafana environment.

## Reference architecture

- [Reference architecture overview](./reference-architecture/overview.md)
- [Adopt the reference architecture](./reference-architecture/adoption.md)
- [Implementation profiles](./implementation-profiles/README.md)
- [Prometheus, Loki, Grafana, and Alloy](./implementation-profiles/prometheus-loki-grafana-alloy.md)
- [OpenBao Operator companion profile](./implementation-profiles/openbao-operator.md)
- [OpenBao Operator integration contract](./implementation-profiles/openbao-operator-integration-contract.md)

## Operate

- [Run the Docker Compose stack](./docker-compose.md)
- [Configure a secure metrics scrape](./metrics/secure-metrics-scrape.md)
- [Configure an all-node metrics scrape](./metrics/all-node-metrics-scrape.md)
- [Configure declarative audit devices](./audit/declarative-audit.md)
- [Design an audit archive path](./audit/audit-archive-reference-design.md)

## Understand

- [OpenBao observability model](./concepts/openbao-observability-model.md)
- [Metrics, logs, and audit logs](./concepts/metrics-vs-logs-vs-audit-logs.md)
- [High-cardinality and label safety](./concepts/high-cardinality-and-label-safety.md)
- [Active-node and all-node observability](./concepts/active-node-vs-all-node-observability.md)
- [OpenBao HA/Raft observability](./concepts/openbao-ha-raft-observability.md)
- [Namespaces and scale observability](./concepts/namespaces-and-scale-observability.md)
- [Audit logs as security records](./concepts/audit-logs-as-security-records.md)
- [Token and lease observability](./concepts/token-and-lease-observability.md)

## Metrics

- [Understanding OpenBao metrics](./metrics/understanding-openbao-metrics.md)
- [Metric compatibility matrix](./metrics/compatibility-matrix.md)
- [OpenBao HA/Raft metrics](./metrics/ha-raft-metrics.md)
- [OpenBao token and lease metrics](./metrics/token-and-lease-metrics.md)

## Logging

- [Understanding OpenBao logs](./logging/understanding-openbao-logs.md)
- [Loki label strategy for OpenBao](./logging/loki-label-strategy.md)
- [Log retention and access control](./logging/retention-and-access-control.md)
- [Audit archive reference design](./audit/audit-archive-reference-design.md)

## Dashboards

- [OpenBao overview dashboard](./dashboards/overview-dashboard.md)
- [OpenBao HA/Raft dashboard](./dashboards/ha-raft.md)
- [OpenBao audit overview dashboard](./dashboards/audit-overview.md)
- [OpenBao audit investigation dashboard](./dashboards/audit-investigation.md)
- [OpenBao operational logs dashboard](./dashboards/operational-logs.md)
- [OpenBao auth and identity dashboard](./dashboards/auth-identity.md)
- [OpenBao token and lease lifecycle dashboard](./dashboards/token-lease-lifecycle.md)
- [OpenBao database secrets dashboard](./dashboards/database-secrets.md)
- [OpenBao Transit dashboard](./dashboards/transit.md)
- [OpenBao PKI dashboard](./dashboards/pki.md)
- [OpenBao secret engines and mounts dashboard](./dashboards/secret-engines-mounts.md)
- [OpenBao runtime and storage dashboard](./dashboards/runtime-storage.md)
- [OpenBao namespaces and scale dashboard](./dashboards/namespaces-scale.md)
- [OpenBao Kubernetes platform dashboard](./dashboards/kubernetes-platform.md)
- [OpenBao SLO and availability dashboard](./dashboards/slo-availability.md)

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
- [Audit archive degraded](./runbooks/audit-archive-degraded.md)
- [Operational log stream missing](./runbooks/operational-log-stream-missing.md)
- [Debug logging enabled](./runbooks/debug-logging-enabled.md)
- [Completed request logging enabled](./runbooks/completed-request-logging-enabled.md)
- [Irrevocable leases present](./runbooks/irrevocable-leases.md)
- [Runtime and storage warnings](./runbooks/runtime-storage-warnings.md)
- [Secret engine feature warnings](./runbooks/secret-engine-feature-warnings.md)
- [Kubernetes platform health](./runbooks/kubernetes-platform-health.md)
- [SLO and availability](./runbooks/slo-availability.md)
- [Security audit detections](./runbooks/security-audit-detections.md)
