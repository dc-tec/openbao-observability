# OpenBao Operator companion profile

Use this explainer when you operate OpenBao with
[`dc-tec/openbao-operator`](https://github.com/dc-tec/openbao-operator) and
want to apply this observability reference architecture to those clusters. It
is for operators who need to connect operator-managed lifecycle signals with
OpenBao workload metrics, logs, audit logs, dashboards, alerts, and runbooks.

Use [OpenBao Operator integration contract](./openbao-operator-integration-contract.md)
when you need the concrete resource, label, scrape, dashboard, alert, and log
contract between the operator and this repository.

## Companion boundary

OpenBao Operator manages the Kubernetes lifecycle for OpenBao clusters. This
repository defines the observability reference architecture for the OpenBao
workloads that the operator creates and manages.

Keep the projects complementary:

| Project | Primary responsibility |
| ------- | ---------------------- |
| `dc-tec/openbao-operator` | Kubernetes lifecycle, CRDs, tenancy, hardened profiles, TLS, unseal, self-init, backups, restores, upgrades, read scaling, and operator control-plane metrics. |
| `openbao-observability` | OpenBao workload signal contracts, metrics and log semantics, audit-log handling, generated dashboards, alerts, runbooks, fixtures, and validation. |

The operator exposes operator control-plane metrics and can render OpenBao
workload telemetry configuration. This reference architecture focuses on the
workload telemetry and the operational evidence around it.

## Surfaces to observe

Observe both the operator and the OpenBao workload, but keep the failure domains
separate.

| Surface | Source | Use it for |
| ------- | ------ | ---------- |
| Operator control plane | Operator `/metrics`, Kubernetes Events, controller logs, and `OpenBaoCluster` status. | Reconcile health, lifecycle drift, upgrade progress, backup freshness, restore state, tenant onboarding, and admission guardrail health. |
| OpenBao workload | `/v1/sys/metrics`, operational logs, audit logs, audit archive, and platform state. | OpenBao availability, seal state, leadership, HA/Raft health, request latency, token and lease pressure, audit health, and security investigations. |
| Platform context | Pod, node, volume, network, ServiceMonitor, and collector signals. | Root-cause analysis when either the operator or OpenBao workload cannot converge. |

Do not collapse these into one health signal. A healthy operator can manage a
sealed or leaderless OpenBao cluster, and a serving OpenBao cluster can still
hide a stalled backup, restore, or upgrade controller path.

## Adoption model

Use the operator as the Kubernetes delivery path, then use this repository to
define and validate the OpenBao workload observability contract.

1. Deploy OpenBao through the operator using the security profile, TLS mode,
   unseal mode, self-init settings, backup settings, and tenancy model that fit
   your environment.

2. Enable OpenBao workload telemetry through the operator's
   `OpenBaoCluster` observability configuration.

3. Choose the scrape profile:

   - Use [Secure metrics scrape](../metrics/secure-metrics-scrape.md) for the
     authenticated active-node baseline.
   - Use [All-node metrics scrape](../metrics/all-node-metrics-scrape.md) when
     you need standby, sealed-node, follower, or per-node Raft visibility and
     have approved network isolation.

4. Configure log collection so OpenBao operational logs, completed request logs,
   audit logs, and audit archive delivery remain separate.

5. Deploy generated Prometheus, Loki, and Grafana artifacts from this repository
   through your platform pipeline.

6. Keep operator alerts and OpenBao workload alerts separate, then connect them
   in dashboards and runbooks.

## Operator integration checklist

An operator-managed cluster fits this profile when these integration points are
available in the platform:

| Capability | Required for | Operator relationship |
| ---------- | ------------ | --------------------- |
| OpenBao telemetry stanza with Prometheus retention | Active-node and all-node metrics | The operator should render this from `spec.observability.metrics` and optional `spec.telemetry`. |
| Stable Kubernetes service registration labels | Active-node scrape | The operator should keep OpenBao Kubernetes service registration enabled so the active pod can be selected. |
| Metrics Service and `ServiceMonitor` or equivalent scrape object | Active-node and all-node metrics | The operator can own this directly, or the platform can apply this repository's scrape manifests with operator pod labels. |
| Metrics token Secret or private metrics-only listener | Secure active scrape or private all-node scrape | Use a scoped token for the secure active-node baseline. Use a private metrics-only listener only after network isolation review. |
| Metrics listener port and NetworkPolicy ingress | Private all-node scrape | The operator needs first-class support for a dedicated metrics listener, pod port, Service port, and restricted ingress. |
| Declarative audit devices | Audit dashboards, alerts, and investigation | The operator should render `spec.audit`; the platform still owns audit collection, retention, and archive controls. |

If the operator version you run does not own a workload `ServiceMonitor`, apply
the scrape manifests from this repository as platform resources and make the
selectors match the operator-managed pod labels. If the operator version does
not expose a dedicated metrics-only listener, use the secure active-node scrape
as the supported baseline and treat all-node scraping as a future operator
integration item.

Use the [OpenBao Operator integration contract](./openbao-operator-integration-contract.md)
for the expected labels, Service shapes, scrape profile behavior, log stream
boundaries, dashboard ownership, and alert ownership.

## Dashboard relationship

Use two dashboard families:

| Dashboard family | Questions it answers |
| ---------------- | -------------------- |
| Operator dashboards | Is the controller reconciling, are backups fresh, is an upgrade active, are read replicas registered, and are cluster status conditions clean. |
| OpenBao workload dashboards | Is OpenBao reachable, unsealed, leader-elected, Raft-healthy, audit-healthy, low-latency, and within token, lease, runtime, and storage expectations. |

This repository provides the OpenBao workload dashboard family. The operator
project can link to these dashboards as the workload observability layer, while
keeping operator-control-plane dashboards focused on CRD and reconcile state.

## Alert relationship

Keep the alert names and ownership distinct.

| Alert class | Owner | Examples |
| ----------- | ----- | -------- |
| Operator lifecycle alerts | Platform team that runs the operator. | Reconcile errors, stale backups, upgrade failures, restore lock conflicts, tenant onboarding failures, and read-replica pool degradation. |
| OpenBao workload alerts | OpenBao service owner or security platform team. | Metrics scrape failure, sealed node, no active leader, multiple active nodes, Raft health, audit request failures, audit stream missing, and runtime/storage pressure. |
| Security investigation alerts | Security and secrets platform responders. | Audit canary missing, audit request/response failures, privileged sys mutations, completed request logging enabled, and suspicious audit patterns. |

When an alert fires, the runbook should make the first branch explicit:

```text
Is the operator failing to converge the desired state?
  -> use operator status, Events, and controller metrics.

Is the OpenBao workload unhealthy after convergence?
  -> use OpenBao metrics, logs, audit logs, and workload runbooks.
```

## Integration opportunities

The companion projects can grow together without making either project own the
other's responsibilities.

- Link the operator observability page to this reference architecture for
  workload telemetry, audit-log handling, and generated dashboards.
- Publish compatibility guidance that pairs operator release validation with the
  OpenBao versions covered by this repository's fixtures.
- Add an operator-managed example that enables workload telemetry and applies
  this repository's generated Prometheus rules and dashboards.
- Add dashboard links from operator dashboards to OpenBao workload dashboards
  by cluster and namespace.
- Add runbook cross-links so operator lifecycle alerts can point to workload
  runbooks when the managed OpenBao cluster is the failing surface.
- Keep release bundles separate: the operator publishes deployment artifacts,
  and this repository publishes observability artifacts.

## Production guidance

For production operator-managed clusters:

- Use the operator's hardened profile and production checklist before you route
  tenant traffic.
- Enable workload telemetry deliberately for each `OpenBaoCluster`.
- Keep the secure active-node scrape as the baseline.
- Add private all-node scraping only after network isolation and security review.
- Restrict audit logs separately from operator logs and ordinary workload logs.
- Validate generated dashboard and alert queries against an operator-managed
  staging cluster before you publish them to production.
- Keep operator compatibility, OpenBao compatibility, and observability fixture
  compatibility visible in release notes.

## What's next

- Use [Reference architecture overview](../reference-architecture/overview.md)
  to understand the portable signal model.
- Use [Adopt the reference architecture](../reference-architecture/adoption.md)
  to map the architecture into your platform.
- Use [Prometheus, Loki, Grafana, and Alloy](./prometheus-loki-grafana-alloy.md)
  when you want to deploy the generated artifacts directly.
- Use [OpenBao Operator integration contract](./openbao-operator-integration-contract.md)
  when you need the concrete resource and label contract for operator-managed
  OpenBao clusters.
- Use [Operator-managed OpenBao examples](../../examples/kubernetes/operator-managed/)
  when you need patch-based examples for active scraping, all-node scraping,
  audit devices, and generated artifact adoption.
- Use [OpenBao Operator observability](https://dc-tec.github.io/openbao-operator/docs/user-guide/openbaocluster/configuration/observability)
  for operator-side metrics and workload telemetry configuration.
- Use [OpenBao Operator production checklist](https://dc-tec.github.io/openbao-operator/docs/operate/production-checklist)
  before you route production traffic to an operator-managed cluster.

Source: This profile reflects the public
[`dc-tec/openbao-operator`](https://github.com/dc-tec/openbao-operator)
README and its operator observability, compatibility, production checklist, and
operator invariants documentation.
