# Configure a secure metrics scrape

Use this how-to to configure an authenticated OpenBao metrics scrape for a
Kubernetes HA deployment. The profile scrapes the active OpenBao node over TLS
with a scoped OpenBao token.

## Before you begin

- Run OpenBao outside development mode.
- Use TLS on the OpenBao listener that serves metrics.
- Install Prometheus Operator if you use the `ServiceMonitor` example.
- Enable Kubernetes service registration or provide an equivalent active
  Service that only selects the active OpenBao pod.
- Get an OpenBao token that can create policies and child tokens.
- Store the OpenBao serving CA certificate in a file named `ca.crt`.

## Choose the scrape profile

| Profile | Use when | Authentication | Target | Caveat |
| ------- | -------- | -------------- | ------ | ------ |
| Authenticated active scrape | You need the secure default for production. | Required OpenBao token. | Active Service. | Standby node state is not visible. |
| Private all-node scrape | You need per-node HA and Raft visibility. | Token or isolated unauthenticated metrics listener. | Each OpenBao pod. | Requires strict network isolation. |
| Local demo scrape | You run the Docker Compose stack in this repo. | None. | All Compose nodes. | Not for production or shared environments. |

Use authenticated active scrape as the baseline. Add a private all-node scrape
only when you have a metrics-only listener and network controls that prevent
general clients from reaching it.

## Configure OpenBao telemetry

1. Enable Prometheus retention in the OpenBao server configuration.

   ```hcl
   telemetry {
     prometheus_retention_time = "30s"
     disable_hostname          = true
   }
   ```

2. Disable metrics on the primary client listener when you use a separate
   metrics listener.

   ```hcl
   listener "tcp" {
     address         = "0.0.0.0:8200"
     cluster_address = "0.0.0.0:8201"
     tls_cert_file   = "/openbao/tls/tls.crt"
     tls_key_file    = "/openbao/tls/tls.key"

     telemetry {
       disallow_metrics = true
     }
   }
   ```

   If you scrape the primary listener instead, leave `disallow_metrics` unset
   and keep `unauthenticated_metrics_access = false`.

3. Use a metrics-only listener when you want to separate client traffic from
   scrape traffic.

   ```hcl
   listener "tcp" {
     address       = "0.0.0.0:8202"
     tls_cert_file = "/openbao/tls/tls.crt"
     tls_key_file  = "/openbao/tls/tls.key"

     telemetry {
       metrics_only                   = true
       unauthenticated_metrics_access = false
     }
   }
   ```

4. Enable Kubernetes service registration when you want an active Service based
   on OpenBao pod labels.

   ```hcl
   service_registration "kubernetes" {}
   ```

   Set `BAO_K8S_NAMESPACE` and `BAO_K8S_POD_NAME` through the Kubernetes
   Downward API when you do not set `namespace` and `pod_name` in the
   `service_registration` stanza.

## Create the metrics policy

1. Write a metrics-only policy.

   ```shell
   cat > openbao-metrics-policy.hcl <<'EOF'
   path "sys/metrics" {
     capabilities = ["read", "list"]
   }
   EOF
   ```

2. Apply the policy.

   ```shell
   bao policy write openbao-metrics openbao-metrics-policy.hcl
   ```

3. Create a token with only the metrics policy.

   ```shell
   OPENBAO_METRICS_TOKEN="$(bao token create \
     -policy=openbao-metrics \
     -field=token)"
   ```

   Use your normal secret-management process to set token TTL, renewal, and
   rotation. Prometheus reads the token; it does not manage the OpenBao token
   lifecycle for you.

4. Store the token in Kubernetes.

   ```shell
   kubectl -n openbao create secret generic openbao-metrics-token \
     --from-literal=token="${OPENBAO_METRICS_TOKEN}"
   ```

5. Store the OpenBao serving CA certificate.

   ```shell
   kubectl -n openbao create configmap openbao-metrics-ca \
     --from-file=ca.crt=./ca.crt
   ```

## Configure the active ServiceMonitor

1. Review the example manifest.

   ```shell
   less examples/kubernetes/secure-metrics-scrape.yaml
   ```

   The manifest creates the active metrics Service and `ServiceMonitor`. It
   references the `openbao-metrics-token` Secret and `openbao-metrics-ca`
   ConfigMap that you created earlier. It also maps deployment identity to
   `cluster`, `kubernetes_namespace`, `pod`, `instance`, and
   `scrape_profile="active"`. OpenBao namespace-scoped metrics use
   `openbao_namespace`.

2. Update the selectors to match your OpenBao deployment labels.

   The example Service selects pods with `openbao-active: "true"`, which
   OpenBao sets when Kubernetes service registration is enabled.

3. Update the `ServiceMonitor` labels so your Prometheus resource selects it.

   Many kube-prometheus-stack installations require a release label such as
   `release: kube-prometheus-stack`.

4. Apply the manifest.

   ```shell
   kubectl apply -f examples/kubernetes/secure-metrics-scrape.yaml
   ```

## Verify the result

1. Check that the active metrics Service has one endpoint.

   ```shell
   kubectl -n openbao get endpoints openbao-active-metrics
   ```

2. Check that Prometheus Operator picked up the `ServiceMonitor`.

   ```shell
   kubectl -n openbao get servicemonitor openbao-active-metrics
   ```

3. Query Prometheus.

   ```promql
   up{job="openbao",scrape_profile="active"}
   ```

   Expected result: one active target with value `1`.

   Confirm that the target has the canonical deployment labels.

   ```promql
   count by (cluster, kubernetes_namespace, pod, instance, scrape_profile) (
     up{job="openbao",scrape_profile="active"}
   )
   ```

4. Check the active node metric.

   ```promql
   vault_core_active
   ```

   Expected result: one series with value `1`.

5. Test the OpenBao endpoint directly from a pod that can reach the Service.

   ```shell
   curl --cacert ca.crt \
     --header "Authorization: Bearer ${OPENBAO_METRICS_TOKEN}" \
     "https://openbao-active-metrics.openbao.svc:8202/v1/sys/metrics?format=prometheus"
   ```

   Expected result: Prometheus text that includes `vault_core_active`.

## Troubleshooting

### Prometheus does not discover the ServiceMonitor

Check the labels on the `ServiceMonitor` and the selector on your Prometheus
resource. Prometheus Operator only includes `ServiceMonitor` objects that match
the Prometheus selector and namespace selector.

### The target returns 403

Check the OpenBao token policy. The token must include access to `sys/metrics`.
Also check that the Kubernetes Secret key name matches the `authorization`
credentials in the `ServiceMonitor`.

### The target has a TLS error

Check the CA certificate and `serverName` in `tlsConfig`. Do not use
`insecureSkipVerify: true` for the production profile.

### The target has ambiguous namespace or cluster labels

Confirm that the ServiceMonitor relabeling matches the example. Keep
`honorLabels: false`. Use `kubernetes_namespace` for workload placement and
`openbao_namespace` for the logical OpenBao namespace. Do not restore the
generic `namespace` label.

### The active Service has no endpoints

Check that Kubernetes service registration is enabled and that OpenBao can
patch its pod labels. The OpenBao service account needs permissions to get,
update, and patch its pod.

## What's next

- Use [OpenBao observability model](../concepts/openbao-observability-model.md)
  to understand how metrics fit with logs, audit logs, and platform signals.
- Use [Active-node and all-node observability](../concepts/active-node-vs-all-node-observability.md)
  to understand what this scrape profile shows and hides.
- Use [Understanding OpenBao metrics](./understanding-openbao-metrics.md) to
  understand source metrics, labels, and recording rules.
- Use [Configure an all-node metrics scrape](./all-node-metrics-scrape.md) when
  you need standby, sealed-node, or per-pod Raft visibility.
- Use [Run the Docker Compose stack](../docker-compose.md) when you need a
  local all-node HA scrape for dashboard validation.
- Use [OpenBao metrics scrape failing](../runbooks/openbao-metrics-scrape-failing.md)
  when the `OpenBaoUnreachable` alert fires.

Source: OpenBao documents the `/sys/metrics` endpoint in the
[OpenBao metrics API documentation][openbao-metrics-api]. OpenBao documents
Prometheus telemetry options in the
[OpenBao telemetry documentation][openbao-telemetry]. OpenBao documents
listener-level metrics controls in the
[OpenBao TCP listener documentation][openbao-tcp-listener]. OpenBao documents
Kubernetes service registration labels in the
[OpenBao Kubernetes service registration documentation][openbao-kubernetes-service-registration].
Prometheus Operator documents `ServiceMonitor` discovery and endpoint
configuration in the
[Prometheus Operator design documentation][prometheus-operator-design] and
[Prometheus Operator API reference][prometheus-operator-api].

[openbao-kubernetes-service-registration]: https://openbao.org/docs/configuration/service-registration/kubernetes/
[openbao-metrics-api]: https://openbao.org/api-docs/system/metrics/
[openbao-tcp-listener]: https://openbao.org/docs/configuration/listener/tcp/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
[prometheus-operator-api]: https://prometheus-operator.dev/docs/api-reference/api/
[prometheus-operator-design]: https://prometheus-operator.dev/docs/getting-started/design/
