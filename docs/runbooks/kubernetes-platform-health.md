# Kubernetes platform health

Use this runbook when a Kubernetes platform alert fires for an OpenBao
workload. These alerts point to pod readiness, restarts, node pressure, PVC
pressure, or collector scrape health.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get access to the OpenBao Kubernetes platform dashboard.
- Get access to Kubernetes for the affected OpenBao namespace.
- Get access to OpenBao operational logs, audit logs, and change history.

## Confirm the alert

1. Check which platform alert fired.

   ```promql
   ALERTS{alertstate="firing", alertname=~"OpenBao(AllPodsUnavailable|AuditPVCPressure|KubernetesPodNotReady|KubernetesPodRestartsIncreasing|KubernetesNodePressure|KubernetesPodSignalMissing|LogCollectorTargetDown|LogCollectorSignalMissing)"}
   ```

2. Open the `OpenBao Kubernetes platform` dashboard.

3. Confirm the dashboard variables match the affected namespace, pod,
   container, PVC, node, and scrape job.

4. Check whether OpenBao workload alerts fired at the same time.

   ```promql
   ALERTS{alertstate="firing", alertname=~"OpenBao(SealedUnexpectedly|NoActiveNode|MultipleActiveNodes|Autopilot.*|Raft.*|Audit.*)"}
   ```

## Investigate pod availability

1. Check OpenBao container readiness by pod.

   ```promql
   min by (cluster, namespace, pod) (
     kube_pod_container_status_ready{pod=~"openbao.*",container=~"openbao|bao"}
   )
   ```

   If this query returns no series, check whether the deployment installs a
   `kubernetes_pods` [signal expectation](../../examples/signal-expectations/).
   `OpenBaoKubernetesPodSignalMissing` fires only for an expected cluster and
   Kubernetes namespace.

2. Check pod phase and recent restarts.

   ```promql
   max by (namespace, pod, phase) (
     kube_pod_status_phase{pod=~"openbao.*"}
   )
   ```

   ```promql
   sum by (cluster, namespace, pod) (
     increase(kube_pod_container_status_restarts_total{pod=~"openbao.*",container=~"openbao|bao"}[15m])
   )
   ```

3. Inspect Kubernetes status for the affected pods.

   ```shell
   kubectl -n <namespace> get pods -l app.kubernetes.io/name=openbao -o wide
   kubectl -n <namespace> describe pod <pod>
   ```

4. Compare with OpenBao status and logs.

   ```shell
   bao status -address=<openbao_address>
   ```

   ```logql
   {log_stream="openbao.operational", namespace="<namespace>", pod="<pod>"}
   ```

## Investigate PVC pressure

1. Check PVC free percentage.

   ```promql
   100 *
   min by (cluster, namespace, persistentvolumeclaim) (
     kubelet_volume_stats_available_bytes{persistentvolumeclaim=~".*(openbao|bao|audit|data).*"}
   )
   /
   min by (cluster, namespace, persistentvolumeclaim) (
     kubelet_volume_stats_capacity_bytes{persistentvolumeclaim=~".*(openbao|bao|audit|data).*"}
   )
   ```

2. Identify whether the affected PVC stores audit logs, Raft data, or another
   OpenBao path.

   ```shell
   kubectl -n <namespace> get pvc
   kubectl -n <namespace> describe pvc <pvc>
   ```

3. Check audit-device health and archive delivery before you rotate or move
   audit files.

   ```promql
   openbao:audit_log_request_failure:increase5m
   openbao:audit_log_response_failure:increase5m
   openbao_audit_archive_delivery_success
   ```

4. Check whether the archive path is healthy enough to tolerate local cleanup.

   ```promql
   time() - max(openbao_audit_archive_last_success_timestamp_seconds)
   sum(increase(openbao_audit_archive_delivery_failures_total[15m]))
   sum(increase(openbao_audit_archive_dead_letter_records_total[15m]))
   ```

## Investigate node pressure

1. Check active node pressure conditions.

   ```promql
   max by (node, condition) (
     kube_node_status_condition{condition=~"DiskPressure|MemoryPressure|PIDPressure",status="true"}
   )
   ```

2. Locate affected OpenBao pods on pressured nodes.

   ```shell
   kubectl -n <namespace> get pods -l app.kubernetes.io/name=openbao -o wide
   kubectl describe node <node>
   ```

3. Check whether the affected node also hosts Prometheus, Loki, Alloy, storage,
   or other dependencies for the OpenBao observability path.

4. If node pressure coincides with Raft or storage symptoms, use the HA/Raft
   runbook before you restart multiple OpenBao pods.

## Investigate collector health

1. Check collector scrape health.

   ```promql
   up{job=~".*(alloy|grafana-alloy|promtail).*"}
   ```

   If this query returns no series, check whether the deployment installs a
   `log_collector` [signal expectation](../../examples/signal-expectations/).
   `OpenBaoLogCollectorSignalMissing` fires only for an expected collector
   target.

2. Check OpenBao operational and audit stream presence.

   ```logql
   {log_stream="openbao.operational"}
   ```

   ```logql
   {log_stream="openbao.audit"}
   ```

3. Inspect collector pods and logs.

   ```shell
   kubectl -n <collector_namespace> get pods
   kubectl -n <collector_namespace> logs <collector_pod>
   ```

4. Check whether the audit archive path is affected.

   ```promql
   openbao_audit_archive_delivery_success
   ```

## Restore the baseline

1. If pods are unavailable because of a rollout, pause further rollout steps
   and restore one OpenBao pod at a time.

2. If OpenBao is sealed, leaderless, or Raft-unhealthy, use the OpenBao
   workload runbook before you change Kubernetes scheduling.

3. If a PVC is low, expand the volume or restore log rotation and archive
   delivery. Do not delete audit files until the security evidence path is
   approved.

4. If node pressure is active, move unrelated workloads, restore node capacity,
   or drain only after you confirm OpenBao quorum and failure tolerance.

5. If the collector is down, restore the collector, credentials, file mounts,
   and network path. Verify audit and operational streams after the collector
   recovers.

## Verify the result

1. Confirm OpenBao workload containers are ready.

   ```promql
   min by (cluster, namespace, pod) (
     kube_pod_container_status_ready{pod=~"openbao.*",container=~"openbao|bao"}
   )
   ```

2. Confirm restart rate returns to zero.

   ```promql
   sum by (cluster, namespace, pod) (
     increase(kube_pod_container_status_restarts_total{pod=~"openbao.*",container=~"openbao|bao"}[15m])
   )
   ```

3. Confirm PVC free space is above the local threshold.

   ```promql
   100 *
   min(kubelet_volume_stats_available_bytes{persistentvolumeclaim=~".*(openbao|bao|audit|data).*"})
   /
   min(kubelet_volume_stats_capacity_bytes{persistentvolumeclaim=~".*(openbao|bao|audit|data).*"})
   ```

4. Confirm collector scrape targets are up.

   ```promql
   up{job=~".*(alloy|grafana-alloy|promtail).*"}
   ```

5. Wait for the alert window to pass and confirm the alert resolves.

## Troubleshooting

### Metrics are empty

Confirm that kube-state-metrics, kubelet or cAdvisor metrics, and PVC metrics
are scraped by the same metrics backend that evaluates the generated rules.
Some Kubernetes distributions rename jobs, restrict kubelet metrics, or do not
expose PVC volume stats for every storage driver.

Missing-signal alerts require an explicit expectation marker. If the marker is
absent, empty optional platform metrics do not fire an alert. Check the
[signal expectation example](../../examples/signal-expectations/) when the
deployment requires these signals.

### The pod selector misses OpenBao pods

Adjust the dashboard variables and alert selector to match your operator or
Helm labels. Keep selectors bounded to the OpenBao workload.

### The collector target is up but logs are missing

Collector scrape health only proves that Prometheus can scrape the collector.
Check collector pipeline errors, Loki write errors, file permissions, and audit
device paths.

### PVC pressure fires on a data PVC

Treat data PVC pressure as a storage and availability issue. Treat audit PVC
pressure as both a security and availability issue because audit-device write
failures can affect OpenBao requests.

## Related pages

- Use [OpenBao Kubernetes platform dashboard](../dashboards/kubernetes-platform.md)
  to inspect platform context.
- Use [OpenBao Raft and Autopilot health](./raft-autopilot-health.md) when pod
  or node symptoms overlap with HA/Raft alerts.
- Use [Audit request and response failures](./audit-request-response-failures.md)
  when PVC or collector symptoms affect audit writes.
- Use [Audit archive degraded](./audit-archive-degraded.md) when durable archive
  delivery is stale, failing, or dead-lettering.
- Use [OpenBao Operator companion profile](../implementation-profiles/openbao-operator.md)
  to keep operator control-plane alerts separate from OpenBao workload alerts.

Source: Kubernetes documents kube-state-metrics as an add-on for Kubernetes
object state in the [Kubernetes kube-state-metrics documentation][kubernetes-ksm].
The kube-state-metrics project documents pod readiness and restart metrics in
its [pod metric reference][kube-state-metrics-pod], and node pressure condition
metrics in its [node metric reference][kube-state-metrics-node]. Kubernetes
documents kubelet node, pod, container, and volume metrics in
[Node metrics data][kubernetes-node-metrics].

[kubernetes-ksm]: https://kubernetes.io/docs/concepts/cluster-administration/kube-state-metrics/
[kubernetes-node-metrics]: https://kubernetes.io/docs/reference/instrumentation/node-metrics/
[kube-state-metrics-node]: https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/cluster/node-metrics.md
[kube-state-metrics-pod]: https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md
