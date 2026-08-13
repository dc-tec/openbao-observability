# OpenBao Operator integration contract

Use this reference when you connect OpenBao clusters managed by
`dc-tec/openbao-operator` to this observability reference architecture. It
defines the workload telemetry, resource, label, dashboard, alert, and log
contract that lets the operator and this repository stay complementary.

## Contract scope

This contract covers observability for OpenBao workloads that the operator
creates. It does not cover operator control-plane metrics,
controller dashboards, backup controller alerts, restore workflows, upgrade
workflows, or tenant admission signals.

Use this contract when you need one of these outcomes:

- The operator renders OpenBao workload telemetry in a way this repository can
  consume.
- This repository publishes dashboards, alerts, and runbooks that work against
  operator-managed clusters.
- Platform teams can decide which resources belong to the operator and which
  resources belong to the observability delivery pipeline.

Validate field availability against the operator version you run. If an
operator version uses different API field names, preserve the behavior in this
contract rather than the literal field path.

## Ownership model

Keep ownership separate so an operator upgrade does not silently redefine
workload observability semantics.

| Area | Owner | Contract expectation |
| ---- | ----- | -------------------- |
| OpenBao lifecycle | OpenBao Operator | The operator owns StatefulSets, Services, TLS wiring, unseal mode, self-init, backups, restores, upgrades, read replicas, and `OpenBaoCluster` status. |
| Workload telemetry configuration | OpenBao Operator | The operator renders OpenBao telemetry, metrics listener configuration, metrics Services, and optional scrape resources from `OpenBaoCluster` configuration. |
| OpenBao signal semantics | This repository | This repository owns metric, log, audit, dashboard, alert, and runbook intent for the OpenBao workload. |
| Artifact delivery | Platform pipeline | The platform applies generated Prometheus, Loki, and Grafana artifacts from this repository or ports their intent to another backend. |
| Operator control-plane observability | OpenBao Operator | Operator dashboards and alerts cover reconciliation, backups, restores, upgrades, tenant onboarding, and read-replica lifecycle. |

Do not treat operator health as OpenBao workload health. A healthy controller
can manage a sealed or leaderless OpenBao cluster, and a serving OpenBao
cluster can still have a stalled backup, restore, upgrade, or read-replica
workflow.

## OpenBaoCluster configuration contract

The operator-facing configuration must let you express these OpenBao workload
observability decisions.

| Concern | Expected operator surface | Required behavior |
| ------- | ------------------------- | ----------------- |
| Enable workload metrics | `spec.observability.metrics.enabled` | Render OpenBao Prometheus telemetry and create the workload metrics Service when enabled. |
| Choose scrape profile | `spec.observability.metrics.scrapeProfile` | Support `Active` for the secure baseline and `AllNodes` for per-node HA/Raft visibility. |
| Configure a metrics-only listener | `spec.observability.metrics.metricsOnlyListener` | Render a dedicated OpenBao listener for metrics collection when all-node scraping or listener separation requires it. |
| Control metrics listener access | `metricsOnlyListener.unauthenticatedMetricsAccess` | Allow unauthenticated metrics only for a private metrics path with network isolation. |
| Create scrape resources | `spec.observability.metrics.serviceMonitor` | Create a Prometheus Operator `ServiceMonitor` when the platform uses Prometheus Operator. |
| Configure scrape authentication | `serviceMonitor.authorization.credentialsSecret` | Reference a Secret containing a scoped OpenBao token for authenticated `/v1/sys/metrics` access. |
| Configure scrape TLS | `serviceMonitor.tlsConfig` | Reference a CA ConfigMap or Secret and set `serverName` when TLS validates a service DNS name. |
| Configure metric prefix | `spec.telemetry.metricsPrefix` | Allow either the default `vault` source prefix or an explicit `openbao` prefix. |
| Configure Prometheus retention | `spec.telemetry.prometheusRetentionTime` | Retain OpenBao metrics long enough for the scrape interval. |
| Configure declarative audit devices | `spec.audit[]` | Render file, syslog, socket, or HTTP audit devices without requiring imperative post-start API calls. |
| Configure read replicas | `spec.readReplicas` | Expose enough workload and status context for read-replica dashboards and alerts to separate quorum health from read capacity. |

Keep the secure active scrape as the default production baseline. Add all-node
scraping only when the platform explicitly needs standby, sealed-node, follower,
read-replica, or per-node runtime visibility.

## Metrics resource contract

The operator can own the Kubernetes resources that expose OpenBao workload
metrics, or the platform can apply equivalent resources. Either path must
preserve the same labels, target shape, and scrape semantics.

| Resource | Expected shape | Required behavior |
| -------- | -------------- | ----------------- |
| Metrics Service | `<cluster-name>-metrics` in the `OpenBaoCluster` namespace. | Expose port `https-metrics` and select the correct pods for the chosen scrape profile. |
| Active metrics Service | ClusterIP Service that selects the active OpenBao pod. | Select `openbao-active: "true"` when Kubernetes service registration supplies that label. |
| All-node metrics Service | Headless Service with `publishNotReadyAddresses: true`. | Select every OpenBao pod configured to expose metrics, including sealed or not-yet-ready pods. |
| ServiceMonitor | `<cluster-name>-metrics` in the `OpenBaoCluster` namespace. | Scrape `/v1/sys/metrics` with `format=prometheus` from the metrics Service. |
| ServiceMonitor endpoint | Port `https-metrics`, path `/v1/sys/metrics`, and `format=prometheus`. | Include interval, timeout, authorization, TLS, and signal-identity relabeling based on the operator configuration. |
| Target relabeling | Prometheus target labels `cluster`, `kubernetes_namespace`, `pod`, and `scrape_profile`. | Preserve deployment identity without accepting conflicting labels from the scraped endpoint. |
| All-node relabeling | Prometheus target labels `pod` and `node`. | Preserve pod and node context for HA/Raft, runtime, and read-replica diagnostics. |
| Metric relabeling | Prometheus metric label `openbao_namespace`. | Copy the OpenBao-generated `namespace` label to `openbao_namespace`, then remove the ambiguous source label. |

Use `ServiceMonitor` when Prometheus Operator is your platform standard. If you
use plain Prometheus, VictoriaMetrics, Grafana Agent, Grafana Alloy, or another
collector, preserve the same endpoint, parameters, labels, and target profile.

## Prometheus signal identity

Use one label name for each identity dimension. Do not use `namespace` for both
Kubernetes placement and an OpenBao logical namespace.

| Prometheus label | Meaning | Source |
| ---------------- | ------- | ------ |
| `cluster` | Platform-assigned OpenBao deployment identity. | The `openbao.org/cluster` Service label for operator-managed resources. |
| `kubernetes_namespace` | Namespace that contains the OpenBao workload. | Kubernetes discovery metadata. |
| `openbao_namespace` | Logical OpenBao namespace when the source metric includes one. | The OpenBao-generated `namespace` metric label. |
| `pod` | OpenBao pod identity. | Kubernetes discovery metadata. |
| `instance` | Prometheus scrape endpoint identity. | The Prometheus default derived from `__address__`. |
| `scrape_profile` | `active` or `all_nodes`. | The selected operator scrape profile. |

Keep `honorLabels` disabled. The collector must own `cluster`,
`kubernetes_namespace`, `pod`, `instance`, and `scrape_profile`. OpenBao must not
replace these labels with values from the metrics endpoint.

OpenBao emits its logical namespace as `namespace` on some metrics. The
ServiceMonitor must copy that value to `openbao_namespace` through metric
relabeling. It must then remove the ambiguous `namespace` label. Metrics that do
not have OpenBao namespace scope must not get a synthetic `openbao_namespace`
value.

The operator-generated ServiceMonitor must implement this identity contract. If
your operator release does not render these relabeling rules, use a
platform-owned equivalent scrape resource. Do not patch an operator-owned
ServiceMonitor because operator reconciliation can remove the patch.

## Label contract

Use labels for stable source identity and routing. Do not use labels to expose
request paths, secret paths, token accessors, entity identifiers, auth
accessors, client addresses, or unbounded policy names.

| Label | Expected value | Applies to | Purpose |
| ----- | -------------- | ---------- | ------- |
| `app.kubernetes.io/name` | `openbao` | Workload resources. | Identifies the application. |
| `app.kubernetes.io/instance` | `OpenBaoCluster` name. | Workload resources. | Identifies the cluster instance. |
| `app.kubernetes.io/managed-by` | `openbao-operator` | Operator-managed resources. | Identifies the lifecycle owner. |
| `openbao.org/cluster` | `OpenBaoCluster` name. | Operator-managed OpenBao resources. | Provides a stable cluster identity for selectors and dashboards. |
| `app.kubernetes.io/component` | `metrics` on metrics resources. | Metrics Service and scrape resources. | Distinguishes metrics exposure from API Services. |
| `openbao.org/component` | `metrics` on metrics resources. | Metrics Service and scrape resources. | Gives operator-owned resources an OpenBao-specific component label. |
| `openbao.org/scrape-profile` | `Active` or `AllNodes`. | Metrics Service and scrape resources. | Supplies the source value for the normalized Prometheus `scrape_profile` label. |
| `openbao.org/workload-pool` | `voter` or `read-replica` when used. | Workload pods and Services. | Separates quorum participants from read-replica pools. |
| `openbao-active` | `"true"` on the active OpenBao pod. | Active scrape selector. | Lets the active metrics Service target exactly one active pod. |

Prometheus and Loki labels do not need to match every Kubernetes label. Promote
only the bounded dimensions needed for routing, dashboards, and incident
triage.

## Active scrape contract

The `Active` scrape profile is the production baseline.

| Requirement | Contract |
| ----------- | -------- |
| Target count | One target per OpenBao cluster. |
| Target selection | Active OpenBao pod only. |
| Authentication | Prefer a scoped OpenBao token with access to `sys/metrics`. |
| Listener | Use the API listener or a dedicated authenticated metrics-only listener. |
| Service shape | ClusterIP Service that does not publish not-ready addresses. |
| Best dashboard coverage | Overview, audit health, token and lease health, request health, and high-level HA state. |
| Known limitation | Standby, sealed-node, follower, read-replica, and per-node runtime detail is incomplete. |

Use this profile when you need the lowest-risk metrics exposure model. It is
also the right fallback when an older operator version or platform profile does
not support a private all-node listener.

## All-node scrape contract

The `AllNodes` scrape profile is an advanced profile for HA/Raft, runtime, and
read-replica diagnostics.

| Requirement | Contract |
| ----------- | -------- |
| Target count | One target per selected OpenBao pod. |
| Target selection | All OpenBao pods in the selected workload pool. |
| Listener | Dedicated metrics-only listener on every selected pod. |
| Service shape | Headless Service with `publishNotReadyAddresses: true`. |
| Standby behavior | Standby metrics need a private metrics path that OpenBao permits on standby nodes. |
| Network control | Restrict the metrics listener to Prometheus or an equivalent collector path. |
| Best dashboard coverage | HA/Raft, runtime/storage, read-replica, sealed-node, standby, and per-node diagnostics. |

If you enable unauthenticated metrics access for all-node scraping, network
isolation becomes part of the security boundary. Use NetworkPolicy, private
routing, firewall rules, mTLS proxying, or sidecar-local scraping so ordinary
clients cannot reach the metrics-only listener.

## Audit and log contract

The operator can configure audit devices, but the platform still owns log
collection, audit archive delivery, retention, and access control.

| Stream | Source | Contract |
| ------ | ------ | -------- |
| `openbao.operational` | OpenBao container logs. | Keep separate from operator controller logs. |
| `openbao.completed_requests` | OpenBao completed request logs when enabled. | Treat as temporary troubleshooting data, not as an audit-log replacement. |
| `openbao.audit` | OpenBao audit devices used for investigation. | Restrict access and preserve request/response entries for security workflows. |
| `openbao.audit_archive` | Compliance or long-term audit archive path. | Keep separate from short-term Loki or dashboard exploration. |
| Operator logs | Operator controller and provisioner logs. | Keep in operator-owned dashboards and runbooks. |

For file audit devices, the audit path must be stable enough for the collector
to tail after pod restarts. If the operator renders audit devices from
`spec.audit[]`, the platform must still provision the volume, file permissions,
collector mount, archive path, and access policy.

Leave ordinary OpenBao operational logs on stderr/stdout for Kubernetes
workloads unless you deliberately mount and manage a writable log volume. A
configured operational log file without a compatible mount can prevent OpenBao
from starting. This is separate from file audit devices, which need explicit
storage and access controls because they contain security records.

## Dashboard contract

Use dashboard families that keep control-plane and workload questions distinct.

| Dashboard family | Owner | Use it for |
| ---------------- | ----- | ---------- |
| Operator dashboards | OpenBao Operator | Reconcile health, controller errors, backup freshness, restore state, upgrade progress, read-replica lifecycle, and CR status. |
| OpenBao workload dashboards | This repository | OpenBao availability, seal state, active node count, request latency, HA/Raft health, runtime pressure, token and lease pressure, audit health, and security investigation. |
| Platform dashboards | Platform team | Pod readiness, restarts, node pressure, PVC pressure, NetworkPolicy reachability, collector health, and Prometheus target health. Use the generated Kubernetes platform dashboard as the reference workload-context view. |

When you link dashboards together, pass bounded context such as `cluster`,
`kubernetes_namespace`, `openbao_namespace`, `pod`, `node`, `scrape_profile`,
and source prefix. Do not pass request paths, secret paths, token accessors,
entity identifiers, auth accessors, or client addresses as dashboard variables.

## Alert contract

Keep alert ownership clear even when a single incident involves both the
operator and the OpenBao workload.

| Alert class | Owner | First question |
| ----------- | ----- | -------------- |
| Operator lifecycle alerts | Operator or platform team. | Is the controller failing to converge the desired state? |
| OpenBao workload alerts | OpenBao service owner. | Is the OpenBao workload unhealthy after the desired state exists? |
| Security investigation alerts | Security or secrets platform responders. | Is there evidence of audit failure, risky activity, or missing security records? |
| Platform alerts | Platform team. | Is Kubernetes, storage, network, DNS, or collection infrastructure preventing either layer from working? |

Runbooks can link across ownership boundaries, but the alert name and primary
owner must not change. This keeps paging, escalation, and post-incident
review clear.

## Minimum acceptance checklist

An operator-managed OpenBao cluster fits this contract when all of these checks
pass:

- The OpenBao workload has Prometheus telemetry enabled.
- The metrics source prefix is documented as `vault` or `openbao`.
- The active scrape exposes exactly one healthy target per cluster.
- The all-node scrape, when enabled, exposes one target per selected OpenBao
  pod and preserves `pod` and `node` labels.
- Workload metrics preserve `cluster`, `kubernetes_namespace`, `pod`,
  `instance`, and `scrape_profile` target identity.
- OpenBao namespace-scoped metrics use `openbao_namespace`; they do not overload
  the Kubernetes namespace label.
- The metrics listener is authenticated, privately reachable, or both.
- The metrics Service and scrape resource carry stable cluster and scrape
  profile labels.
- OpenBao operational logs and operator logs land in different streams.
- Audit logs land in a restricted stream and have a separate archive decision.
- Generated Prometheus, Loki, and Grafana artifacts from this repository are
  validated against the operator-managed staging cluster.
- Operator control-plane alerts and OpenBao workload alerts route to owners who
  can act on their first diagnostic step.

## Compatibility notes

Raw OpenBao metrics do not expose a consistent cluster label across every
metric family and deployment profile. Use the generated recording rules from
this repository before you make dashboards or alerts depend on normalized
cluster-level signals.

OpenBao deployments commonly emit `vault_*` metrics unless you configure an
`openbao` metrics prefix. This repository generates artifacts for both source
prefixes.

The local OpenBao fixture validates basic Raft non-voter behavior with one
read replica. Operator-managed read replicas still need live Kubernetes
validation before you page on operator-specific role labels. Use all-node
scraping for diagnosis, then keep quorum alerts separate from read-capacity
alerts.

## Related pages

- Use [OpenBao Operator companion profile](./openbao-operator.md) to understand
  the high-level companion model.
- Use [Operator-managed OpenBao examples](../../examples/kubernetes/operator-managed/)
  when you need patch-based examples for active scraping, all-node scraping,
  declarative audit devices, and generated artifact adoption.
- Use [Configure a secure metrics scrape](../metrics/secure-metrics-scrape.md)
  for the active scrape baseline.
- Use [Configure an all-node metrics scrape](../metrics/all-node-metrics-scrape.md)
  for HA/Raft and read-replica diagnostics.
- Use [Metric compatibility matrix](../metrics/compatibility-matrix.md) before
  you rely on feature-specific metrics.
- Use [Namespaces and scale observability](../concepts/namespaces-and-scale-observability.md)
  before you expose namespace or read-replica dimensions broadly.
- Use [OpenBao Kubernetes platform dashboard](../dashboards/kubernetes-platform.md)
  for workload pod, PVC, node, collector, and Kubernetes event context.
- Use [Loki label strategy for OpenBao](../logging/loki-label-strategy.md)
  before you promote log fields to labels.
