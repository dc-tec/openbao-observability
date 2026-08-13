---
title: "Dashboards"
description: "Generated Grafana dashboards for overview, HA/Raft, audit, logs, identity, leases, secret engines, runtime, Kubernetes, and SLOs."
weight: 70
browse:
  - "/dashboards/overview-dashboard"
  - "/dashboards/ha-raft"
  - "/dashboards/audit-overview"
  - "/dashboards/audit-investigation"
  - "/dashboards/operational-logs"
  - "/dashboards/auth-identity"
  - "/dashboards/token-lease-lifecycle"
  - "/dashboards/database-secrets"
  - "/dashboards/transit"
  - "/dashboards/pki"
  - "/dashboards/secret-engines-mounts"
  - "/dashboards/runtime-storage"
  - "/dashboards/namespaces-scale"
  - "/dashboards/kubernetes-platform"
  - "/dashboards/slo-availability"
---

# Dashboards

Dashboard guides define each generated Grafana dashboard's purpose, required
data sources, and panel interpretation.

## Topics

- [OpenBao overview dashboard](/dashboards/overview-dashboard/) gives the
  first-stop health view.
- [OpenBao HA/Raft dashboard](/dashboards/ha-raft/) focuses on leadership,
  Raft health, Autopilot, and peer state.
- [OpenBao audit investigation dashboard](/dashboards/audit-investigation/)
  supports request ID drilldown, risky paths, and auth activity.
- [OpenBao namespaces and scale dashboard](/dashboards/namespaces-scale/)
  shows namespace activity, Raft voters, and read-replica signals.
