# Operator-managed kind validation profile

Use this profile to validate this repository's observability artifacts against
an OpenBao cluster that `dc-tec/openbao-operator` manages in kind. This profile
is a consumer validation environment. The operator repository remains the
source of truth for installation, CRD fields, reconciliation behavior, and
status semantics.

## Before you begin

- Install `kind`, `kubectl`, Docker, and Go.
- Generate this repository's artifacts with `make generate`.
- Install the OpenBao Operator by following the operator repository
  instructions.
- Install Prometheus Operator or a compatible monitoring stack that supports
  `ServiceMonitor` and `PrometheusRule` resources.
- Install Loki or configure another Loki-compatible endpoint when you validate
  log dashboards.
- Set `PROMETHEUS_URL` and `LOKI_URL` to endpoints that this repository's
  validation command can reach.

## Example files

| File | Purpose |
| ---- | ------- |
| `kind-config.yaml` | Creates a local kind cluster with one control-plane node and three worker nodes. |
| `namespace.yaml` | Creates the `openbao` namespace used by the local validation profile. |
| `tenant.yaml` | Onboards the `openbao` namespace through `OpenBaoTenant` for operator multi-tenant installs. |
| `kustomization.yaml` | Applies namespace bootstrap resources for the profile. |
| `openbaocluster-observability.patch.yaml` | Enables all-node metrics, the `openbao` metric prefix, and one read replica on an existing `OpenBaoCluster`. |

## Create the kind cluster

Create the cluster from this repository:

```shell
make kind-operator-up
```

Apply the namespace bootstrap resources:

```shell
make kind-operator-apply
```

## Install the operator

Install the OpenBao Operator from the operator repository. For local source
validation, set `OPENBAO_OPERATOR_REPO` to the operator checkout and run the
installation workflow documented by that repository.

If you install from a released manifest or chart, follow the operator
documentation for that release. Keep the operator API version aligned with the
patches in this directory.

## Onboard the validation tenant

Apply the validation tenant after the operator CRDs are installed:

```shell
make kind-operator-apply-tenant
```

This tenant grants the operator permission to manage `OpenBaoCluster` resources
in the `openbao` namespace. If your operator installation uses a central
operator namespace for tenant onboarding, create the equivalent
`OpenBaoTenant` from that namespace and keep `spec.targetNamespace: openbao`.

## Create or patch an OpenBaoCluster

Create a local `OpenBaoCluster` by following the operator repository examples.
Use a development or validation profile, three voter replicas, self-init, and
storage that works on kind.

For this validation profile, keep OpenBao operational logs on stderr/stdout.
Do not set `spec.configuration.logging.file` unless the cluster also mounts a
writable volume at that path. Use `spec.audit[]` for declarative audit devices
instead of generic self-init API requests for audit setup.

Patch the cluster with this repository's observability validation shape:

```shell
kubectl -n openbao patch openbaocluster openbao-observability \
  --type merge \
  --patch-file examples/kubernetes/kind/operator-managed/openbaocluster-observability.patch.yaml
```

Patch the kind API server endpoint IPs into the `OpenBaoCluster`:

```shell
make kind-operator-patch-api-server-endpoints
```

This target reads the current Kubernetes API endpoint IPs from the cluster and
patches `spec.network.apiServerEndpointIPs`. It avoids hardcoding kind control
plane addresses in versioned examples.

Override the resource names when your local cluster uses different names:

```shell
make kind-operator-validate \
  KIND_OPERATOR_NAMESPACE=openbao \
  KIND_OPERATOR_OPENBAO_CLUSTER=openbao-observability \
  KIND_OPERATOR_TENANT_NAMESPACE=openbao \
  KIND_OPERATOR_TENANT=openbao-observability
```

## Apply generated rules

Apply the generated Prometheus Operator rules that match the `openbao` source
prefix configured by the patch:

```shell
make kind-operator-apply-rules \
  KIND_OPERATOR_PROMETHEUS_RULE_NAMESPACE=monitoring \
  KIND_OPERATOR_RULE_PROFILE=openbao-prefix
```

Use `KIND_OPERATOR_RULE_PROFILE=vault-prefix` when your `OpenBaoCluster`
emits the default `vault_*` metric names.

## Validate the result

Run the local validation checks after Prometheus and Loki can reach the
operator-managed OpenBao workload:

```shell
make kind-operator-validate \
  PROMETHEUS_URL=http://127.0.0.1:19090 \
  LOKI_URL=http://127.0.0.1:13100
```

The target checks that:

- the `OpenBaoCluster` CRD exists;
- the validation `OpenBaoTenant` exists and reports `status.provisioned=true`;
- the named `OpenBaoCluster` exists;
- the operator reports the cluster as available;
- at least one workload `ServiceMonitor` exists for the cluster; and
- generated dashboard queries validate against the configured Prometheus and
  Loki endpoints.

## Troubleshooting

### OpenBao pods cannot reach the Kubernetes API

Run `make kind-operator-api-server-endpoints` and confirm it prints at least
one endpoint IP. Then run `make kind-operator-patch-api-server-endpoints`.

Some CNI implementations enforce egress after service VIP translation. In
those environments, allowing only the Kubernetes service IP is not enough for
OpenBao service registration or Kubernetes auth flows.

### OpenBao exits after enabling file logging

Remove `spec.configuration.logging.file` or mount a writable volume at the
configured path. Kubernetes profiles should normally collect OpenBao
operational logs from stdout/stderr.

### ServiceMonitor creation fails

Confirm that the operator tenant RBAC can manage `ServiceMonitor` resources in
the validation namespace. The operator repository owns that RBAC model. This
repository only validates that generated dashboards and alerts work after the
scrape resource exists.

## Validation boundary

This profile validates the observability contract from this repository. It does
not define operator semantics.

| This repository validates | Operator repository defines |
| ------------------------- | --------------------------- |
| Generated recording rules, alerts, dashboards, and query compatibility. | Operator installation and upgrade flows. |
| Scrape-profile expectations for active-node and all-node visibility. | `OpenBaoCluster` API fields and defaulting. |
| Read-replica observability labels and dashboard behavior. | Read-replica lifecycle, status conditions, and reconciliation rules. |
| Log stream, audit stream, and dashboard expectations. | Workload rendering, ownership, and day-2 behavior. |

## Clean up

Delete the kind cluster when you finish validation:

```shell
make kind-operator-down
```

## Related pages

- Use [Operator-managed OpenBao examples](../../operator-managed/) when you
  adapt active scrape, all-node scrape, and audit patches to an existing
  operator-managed cluster.
- Use [OpenBao Operator companion profile](../../../../docs/implementation-profiles/openbao-operator.md)
  to understand the ownership boundary.
- Use [OpenBao Operator observability](https://dc-tec.github.io/openbao-operator/docs/user-guide/openbaocluster/configuration/observability)
  for operator-side metrics and workload telemetry configuration.
