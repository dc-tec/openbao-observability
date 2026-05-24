# OpenBao Kubernetes platform dashboard

Use this explainer to read the generated OpenBao Kubernetes platform
dashboard. It is for platform and OpenBao operators who need pod, node, PVC,
collector, and Kubernetes event context around an OpenBao workload.

## What this dashboard is for

Use the Kubernetes platform dashboard when an OpenBao alert might have a
Kubernetes cause or when an operator-managed cluster needs platform context.

The dashboard answers these questions:

- Are OpenBao workload containers ready?
- Did OpenBao pods restart recently?
- Can Prometheus scrape OpenBao and the log collector?
- Is the audit or data PVC running out of space?
- Are Kubernetes node pressure conditions active?
- Are CPU or memory signals changing by pod?
- Did Kubernetes events report warning-style symptoms for OpenBao pods?

## What this dashboard is not for

Do not use this dashboard as an operator control-plane dashboard. It does not
show reconcile errors, backup freshness, restore state, upgrade status, or
`OpenBaoCluster` conditions from the OpenBao Operator.

Use operator dashboards for controller behavior. Use this dashboard for the
Kubernetes runtime context around the OpenBao workload after the desired state
exists.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | kube-state-metrics, kubelet or cAdvisor metrics, OpenBao scrape target health, and collector scrape target health. |
| Loki | `loki` | Kubernetes platform events collected with `log_stream="platform.kubernetes"`. |

Kubernetes metric names and labels vary by monitoring distribution. Validate
the dashboard against your platform before you use it for paging decisions.

## Dashboard variables

The dashboard exposes these variables:

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| Namespace | `.*` | Selects Kubernetes namespaces. |
| Pod | `openbao.*` | Selects OpenBao workload pods. |
| Container | `openbao|bao` | Selects OpenBao workload containers. |
| PVC | `.*(openbao|bao|audit|data).*` | Selects OpenBao audit or data PVCs. |
| Node | `.*` | Selects Kubernetes nodes. |
| OpenBao scrape job | `openbao.*` | Selects OpenBao Prometheus scrape targets. |
| Collector scrape job | `.*(alloy|grafana-alloy|promtail).*` | Selects log collector scrape targets. |

Use bounded filters in shared dashboards. Do not add workload labels that carry
tenant, secret path, request path, token, or entity metadata.

## How to read workload health

Start with ready containers, unready containers, restarts, and readiness by
pod. Healthy OpenBao pods remain ready and do not restart outside expected
rollouts.

If readiness is unhealthy, compare the timing with OpenBao scrape health,
operational logs, and HA/Raft health. A Kubernetes readiness failure can be a
platform symptom, an OpenBao seal or leadership symptom, or both.

## How to read scrape and collector health

OpenBao scrape health shows whether Prometheus can reach the configured
OpenBao scrape job. Collector scrape health shows whether Prometheus can reach
the configured collector job.

Collector scrape health is not the same as log delivery health. Pair it with
the operational log stream, audit log stream, and audit canary alerts before
you decide whether logs are missing.

## How to read PVC health

Audit PVC free and PVC free by claim show free space for matching OpenBao
audit or data PVCs. Low free space on an audit PVC can become a security and
availability issue because OpenBao audit-device failures can affect request
handling.

Use PVC panels with archive health. A healthy archive does not remove the need
to keep the local audit file writable, and a writable audit file does not prove
that the archive path is healthy.

## How to read node pressure

Node pressure panels show `DiskPressure`, `MemoryPressure`, and `PIDPressure`
conditions. These are platform symptoms that can disrupt OpenBao pods,
collectors, or storage-backed workloads.

Node pressure does not prove OpenBao is faulty. Treat it as context for pod
readiness, restarts, scrape failures, PVC pressure, and operational logs.

## How to read platform events

The platform events panel filters the `platform.kubernetes` stream for
warning-style words such as `failed`, `backoff`, `evict`, `pressure`, and
`probe`.

Use this panel to find the Kubernetes event that explains a pod or node symptom.
Do not treat Kubernetes events as audit evidence.

## Common mistakes

- Treating a healthy operator reconcile loop as proof that OpenBao is healthy.
- Treating OpenBao workload alerts as operator control-plane alerts.
- Paging on Kubernetes metrics in non-Kubernetes environments.
- Assuming every platform exports the same kubelet, cAdvisor, or PVC metrics.
- Using PVC free space as a substitute for audit archive health.
- Adding high-cardinality Kubernetes labels to shared dashboards.

## Known limitations

- The dashboard assumes kube-state-metrics-style pod and node metrics.
- CPU and memory panels assume cAdvisor or kubelet container metrics.
- PVC panels depend on kubelet volume stats and matching PVC labels.
- Event panels depend on a collector that writes Kubernetes events to Loki with
  `log_stream="platform.kubernetes"`.
- The default selectors are conservative and may need adjustment for a
  platform that does not name the OpenBao container `openbao` or `bao`.

## What's next

- Use [Kubernetes platform health](../runbooks/kubernetes-platform-health.md)
  when a platform alert fires.
- Use [OpenBao Operator companion profile](../implementation-profiles/openbao-operator.md)
  to keep workload, platform, and operator signals separate.
- Use [OpenBao Operator integration contract](../implementation-profiles/openbao-operator-integration-contract.md)
  before you rely on operator-managed scrape resources.
- Use [Active-node and all-node observability](../concepts/active-node-vs-all-node-observability.md)
  to choose the OpenBao scrape profile.
- Use [Log retention and access control](../logging/retention-and-access-control.md)
  before you collect Kubernetes events into a shared Loki tenant.

Source: Kubernetes documents kube-state-metrics as an add-on that exposes
Kubernetes object state for Prometheus queries in the
[Kubernetes kube-state-metrics documentation][kubernetes-ksm]. The
kube-state-metrics project documents `kube_pod_container_status_ready`,
`kube_pod_container_status_restarts_total`, and
`kube_node_status_condition` in its [pod][kube-state-metrics-pod] and
[node][kube-state-metrics-node] metric references. Kubernetes documents kubelet
node, pod, container, and volume metrics in [Node metrics
data][kubernetes-node-metrics]. This page describes the generated dashboard
contract in `contracts/dashboards/openbao-kubernetes-platform.yaml`.

[kubernetes-ksm]: https://kubernetes.io/docs/concepts/cluster-administration/kube-state-metrics/
[kubernetes-node-metrics]: https://kubernetes.io/docs/reference/instrumentation/node-metrics/
[kube-state-metrics-node]: https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/cluster/node-metrics.md
[kube-state-metrics-pod]: https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md
