# Configure an all-node metrics scrape

Use this how-to to configure an all-node OpenBao metrics scrape for a
Kubernetes HA deployment. This profile is for Raft, standby, sealed-node, and
per-pod visibility that the secure active scrape does not provide.

> [!WARNING]
> This profile uses unauthenticated access on a dedicated metrics-only listener
> because OpenBao disables `/v1/sys/metrics` on standby nodes unless
> unauthenticated metrics access is enabled. You must restrict the listener to
> Prometheus or an equivalent collector path with NetworkPolicy, firewall rules,
> private routing, mTLS proxying, or sidecar-local scraping.

## Before you begin

- Configure the secure active scrape first, unless this deployment has an
  equivalent authenticated active scrape.
- Run OpenBao outside development mode.
- Use TLS on the metrics listener.
- Install Prometheus Operator if you use the `ServiceMonitor` example.
- Know the labels on your OpenBao server pods.
- Store the OpenBao serving CA certificate in a file named `ca.crt`.
- Confirm that your cluster enforces Kubernetes NetworkPolicy or that you have
  equivalent network controls outside Kubernetes.

## Choose this profile deliberately

Use the all-node profile when you need one of these signals:

- sealed or unsealed state for every OpenBao pod,
- standby scrape health,
- Raft follower visibility,
- per-pod runtime pressure, or
- dashboard validation for HA and Raft panels.

Do not use this profile as a shortcut around token management. Use
[Configure a secure metrics scrape](./secure-metrics-scrape.md) for the
default production scrape.

## Configure the metrics listener

1. Enable Prometheus retention in the OpenBao server configuration.

   ```hcl
   telemetry {
     prometheus_retention_time = "30s"
     disable_hostname          = true
   }
   ```

2. Disable metrics on the primary client listener.

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

3. Add a dedicated metrics-only listener.

   ```hcl
   listener "tcp" {
     address       = "0.0.0.0:8202"
     tls_cert_file = "/openbao/tls/tls.crt"
     tls_key_file  = "/openbao/tls/tls.key"

     telemetry {
       metrics_only                   = true
       unauthenticated_metrics_access = true
     }
   }
   ```

4. Apply the same listener configuration to every OpenBao server.

## Restrict network access

1. Allow Prometheus or your metrics collector to reach only the metrics
   listener port.

2. Preserve the existing OpenBao API and cluster traffic rules.

   Kubernetes NetworkPolicies apply to the selected pod, not only to the port
   you are adding. If you create an ingress NetworkPolicy that selects OpenBao
   pods, include the rules that your API clients, load balancers, peers, and
   metrics collectors need.

3. Confirm that clients outside the metrics collector path cannot connect to
   port `8202`.

4. Confirm that OpenBao nodes can still communicate on the API and cluster
   ports after you apply the network controls.

Source: Kubernetes documents NetworkPolicy as pod-level ingress and egress
controls in the [Kubernetes Network Policies documentation][kubernetes-network-policy].

## Configure the all-node ServiceMonitor

1. Store the OpenBao serving CA certificate.

   ```shell
   kubectl -n openbao create configmap openbao-metrics-ca \
     --from-file=ca.crt=./ca.crt
   ```

2. Review the example manifest.

   ```shell
   less examples/kubernetes/all-node-metrics-scrape.yaml
   ```

   The manifest creates a headless Service and `ServiceMonitor`. The Service
   uses `publishNotReadyAddresses: true` so Prometheus can keep targeting pods
   that are sealed or not ready.

3. Update the Service selector to match your OpenBao server pod labels.

4. Update `tlsConfig.serverName` to match a DNS name in the OpenBao serving
   certificate.

5. Update the `ServiceMonitor` labels so your Prometheus resource selects it.

   Many kube-prometheus-stack installations require a release label such as
   `release: kube-prometheus-stack`.

6. Apply the manifest.

   ```shell
   kubectl apply -f examples/kubernetes/all-node-metrics-scrape.yaml
   ```

## Verify the result

1. Check that the all-node metrics Service has one endpoint per OpenBao pod.

   ```shell
   kubectl -n openbao get endpoints openbao-all-node-metrics
   ```

2. Check that Prometheus Operator picked up the `ServiceMonitor`.

   ```shell
   kubectl -n openbao get servicemonitor openbao-all-node-metrics
   ```

3. Query Prometheus for scrape health.

   ```promql
   up{job="openbao-all-nodes"}
   ```

   Expected result: one series per OpenBao pod with value `1`.

4. Query Prometheus for unsealed node count.

   ```promql
   sum(${p}_core_unsealed{cluster!=""})
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

   Expected result: the number of unsealed OpenBao pods in the cluster.

5. Query Prometheus for active node count.

   ```promql
   sum(${p}_core_active)
   ```

   Expected result: `1`.

## Troubleshooting

### Only one target appears

Check the Service selector. The all-node Service must not select
`openbao-active: "true"` because that label intentionally selects only the
active pod.

### Standby targets return 404 or 403

Check the metrics listener configuration. OpenBao documents standby metrics as
available only when unauthenticated metrics access is enabled.

### Targets have TLS errors

Check the CA certificate and `serverName` in `tlsConfig`. Do not use
`insecureSkipVerify: true` for this profile.

### OpenBao clients lose access after NetworkPolicy changes

Your NetworkPolicy selected the OpenBao pods and omitted required API or
cluster traffic. Restore the previous policy, then reapply a policy that covers
API clients, OpenBao peer traffic, and the metrics collector.

## What's next

- Use [OpenBao observability model](../concepts/openbao-observability-model.md)
  to understand the active-node and all-node scrape tradeoff.
- Use [Active-node and all-node observability](../concepts/active-node-vs-all-node-observability.md)
  to choose between secure active-node and private all-node scraping.
- Use [OpenBao HA/Raft metrics](./ha-raft-metrics.md) to understand the
  per-node Raft and Autopilot signals this profile enables.
- Use [Configure a secure metrics scrape](./secure-metrics-scrape.md) when you
  need the authenticated active-node production baseline.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you group all-node metrics by optional labels.
- Use [Run the Docker Compose stack](../docker-compose.md) when you need a
  local all-node HA scrape for dashboard validation.
- Use [No active OpenBao leader](../runbooks/no-active-openbao-leader.md) when
  the all-node scrape shows no active node.

Source: OpenBao documents Prometheus telemetry behavior in the
[OpenBao telemetry documentation][openbao-telemetry]. OpenBao documents
metrics-only listener controls in the
[OpenBao TCP listener documentation][openbao-tcp-listener]. Prometheus Operator
documents `ServiceMonitor` endpoint configuration in the
[Prometheus Operator API reference][prometheus-operator-api].

[kubernetes-network-policy]: https://kubernetes.io/docs/concepts/services-networking/network-policies/
[openbao-tcp-listener]: https://openbao.org/docs/configuration/listener/tcp/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
[prometheus-operator-api]: https://prometheus-operator.dev/docs/api-reference/api/
