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
| `kustomization.yaml` | Applies observability-owned Kubernetes resources for the profile. |
| `openbaocluster-observability.patch.yaml` | Enables all-node metrics, the `openbao` metric prefix, and one read replica on an existing `OpenBaoCluster`. |

## Create the kind cluster

Create the cluster from this repository:

```shell
make kind-operator-up
```

Apply the namespace and observability-owned resources:

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

## Create or patch an OpenBaoCluster

Create a local `OpenBaoCluster` by following the operator repository examples.
Use a development or validation profile, three voter replicas, self-init, and
storage that works on kind.

Patch the cluster with this repository's observability validation shape:

```shell
kubectl -n openbao patch openbaocluster openbao-observability \
  --type merge \
  --patch-file examples/kubernetes/kind/operator-managed/openbaocluster-observability.patch.yaml
```

Override the resource names when your local cluster uses different names:

```shell
make kind-operator-validate \
  KIND_OPERATOR_NAMESPACE=openbao \
  KIND_OPERATOR_OPENBAO_CLUSTER=openbao-observability
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
- the named `OpenBaoCluster` exists;
- the operator reports the cluster as available;
- at least one workload `ServiceMonitor` exists for the cluster; and
- generated dashboard queries validate against the configured Prometheus and
  Loki endpoints.

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

## What's next

- Use [Operator-managed OpenBao examples](../../operator-managed/) when you
  adapt active scrape, all-node scrape, and audit patches to an existing
  operator-managed cluster.
- Use [OpenBao Operator companion profile](../../../../docs/implementation-profiles/openbao-operator.md)
  to understand the ownership boundary.
- Use [OpenBao Operator observability](https://dc-tec.github.io/openbao-operator/docs/user-guide/openbaocluster/configuration/observability)
  for operator-side metrics and workload telemetry configuration.
