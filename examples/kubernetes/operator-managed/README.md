# Operator-managed OpenBao examples

Use these examples when `dc-tec/openbao-operator` manages your OpenBao
clusters and you want to apply this observability reference architecture to the
OpenBao workload. The examples show merge patches for existing
`OpenBaoCluster` resources, not complete production cluster definitions.

## Example files

| File | Purpose |
| ---- | ------- |
| `metrics-ca-configmap.yaml` | Placeholder ConfigMap for the OpenBao serving CA used by the authenticated active scrape examples. |
| `active-scrape-openbaocluster-patch.yaml` | Enables the secure active-node workload scrape profile. |
| `all-node-scrape-openbaocluster-patch.yaml` | Enables the private all-node workload scrape profile for HA/Raft and per-node diagnostics. |
| `declarative-audit-openbaocluster-patch.yaml` | Adds declarative file audit devices for investigation and archive paths. |

## Before you begin

- Install OpenBao Operator and create a production-ready `OpenBaoCluster`.
- Install Prometheus Operator if you want the operator to render
  `ServiceMonitor` resources.
- Create or rotate a scoped OpenBao token that can read `sys/metrics`.
- Identify the OpenBao serving CA certificate used by the metrics endpoint.
- Confirm the OpenBao metric prefix you want to use: `vault` or `openbao`.
- Review the
  [OpenBao Operator integration contract](../../../docs/implementation-profiles/openbao-operator-integration-contract.md).

## Configure the active scrape baseline

1. Create a Secret that contains the scoped OpenBao metrics token.

   ```shell
   kubectl -n openbao create secret generic openbao-metrics-token \
     --from-literal=token="${OPENBAO_METRICS_TOKEN}"
   ```

2. Copy `metrics-ca-configmap.yaml` into your platform configuration.

3. Replace these placeholder values:

   - `metadata.namespace`
   - `data.ca.crt`

4. Apply the CA ConfigMap to the namespace that contains the `OpenBaoCluster`.

   ```shell
   kubectl apply -f examples/kubernetes/operator-managed/metrics-ca-configmap.yaml
   ```

5. Copy `active-scrape-openbaocluster-patch.yaml` into your platform
   configuration.

6. Replace these placeholder values:

   - `serviceMonitor.labels.release`
   - `serviceMonitor.authorization.credentialsSecret.name`
   - `serviceMonitor.tlsConfig.serverName`
   - `serviceMonitor.tlsConfig.caConfigMap.name`

7. Patch the existing `OpenBaoCluster`.

   ```shell
   kubectl -n openbao patch openbaocluster prod-openbao \
     --type merge \
     --patch-file examples/kubernetes/operator-managed/active-scrape-openbaocluster-patch.yaml
   ```

## Configure the all-node profile

Use the all-node profile only when you need per-node visibility for HA/Raft,
sealed-node diagnostics, standby nodes, read replicas, or runtime pressure.

1. Copy `all-node-scrape-openbaocluster-patch.yaml` into your platform
   configuration.

2. Replace the ServiceMonitor labels and TLS settings for your monitoring
   stack.

3. Confirm that NetworkPolicy or equivalent controls restrict the metrics-only
   listener to Prometheus or an approved collector.

4. Patch the existing `OpenBaoCluster`.

   ```shell
   kubectl -n openbao patch openbaocluster prod-openbao \
     --type merge \
     --patch-file examples/kubernetes/operator-managed/all-node-scrape-openbaocluster-patch.yaml
   ```

## Configure declarative audit devices

Use `declarative-audit-openbaocluster-patch.yaml` when the operator should
configure audit devices as part of the workload baseline.

Before you apply the patch, make sure the OpenBao pods have the volume mounts
and permissions required for the configured file paths. The platform still owns
collection, archive delivery, retention, and access control for audit logs.

```shell
kubectl -n openbao patch openbaocluster prod-openbao \
  --type merge \
  --patch-file examples/kubernetes/operator-managed/declarative-audit-openbaocluster-patch.yaml
```

## Apply generated observability artifacts

Apply the generated PrometheusRule artifacts that match the metric prefix you
configured on the OpenBao workload.

```shell
kubectl -n monitoring apply -f generated/prometheusrules/openbao-prefix/openbao-recording-rules.yaml
kubectl -n monitoring apply -f generated/prometheusrules/openbao-prefix/openbao-alerts.yaml
kubectl -n monitoring apply -f generated/prometheusrules/openbao-prefix/openbao-warning-alerts.yaml
kubectl -n monitoring apply -f generated/prometheusrules/openbao-prefix/openbao-security-alerts.yaml
```

Use the `vault-prefix` generated artifacts instead when the OpenBao workload
emits the default `vault_*` metric names.

Load Grafana dashboards through your normal Grafana delivery path. For Grafana
sidecar deployments, create a labeled ConfigMap from the generated dashboard
JSON files.

```shell
kubectl -n monitoring create configmap openbao-grafana-dashboards \
  --from-file=generated/grafana \
  --dry-run=client \
  -o yaml | kubectl label -f - grafana_dashboard=1 --local -o yaml | kubectl apply -f -
```

## Verify the result

Your operator-managed observability profile is ready for dashboard review when
these checks pass:

- Prometheus discovers the workload `ServiceMonitor`.
- The active scrape has one healthy target per OpenBao cluster.
- The all-node scrape, when enabled, has one healthy target per selected
  OpenBao pod.
- Recording rules evaluate for the selected source prefix.
- OpenBao operational logs and operator logs land in separate streams.
- Audit logs land in a restricted stream and have an archive decision.
- Grafana can load the generated dashboards.

## What's next

- Use [OpenBao Operator companion profile](../../../docs/implementation-profiles/openbao-operator.md)
  to understand the ownership boundary.
- Use [OpenBao Operator integration contract](../../../docs/implementation-profiles/openbao-operator-integration-contract.md)
  to verify labels, scrape resources, dashboards, alerts, and log streams.
- Use [Secure metrics scrape](../../../docs/metrics/secure-metrics-scrape.md)
  for the active scrape model.
- Use [All-node metrics scrape](../../../docs/metrics/all-node-metrics-scrape.md)
  for HA/Raft and per-node diagnostics.
