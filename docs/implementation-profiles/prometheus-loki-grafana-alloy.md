# Prometheus, Loki, Grafana, and Alloy profile

Use this explainer to understand the tested Prometheus, Loki, Grafana, and
Grafana Alloy implementation profile. It is for operators who want to run the
included stack locally, adopt the generated artifacts directly, or port this
profile into an existing observability platform.

## Profile summary

This profile maps the reference architecture to one concrete open source stack.

| Architecture layer | Profile implementation | Repository artifacts |
| ------------------ | ---------------------- | -------------------- |
| Metrics collection | Prometheus scrapes OpenBao `/v1/sys/metrics` with `format=prometheus`. | [Docker Compose Prometheus config](../../examples/docker-compose/prometheus/prometheus.yml) and [Kubernetes scrape examples](../../examples/kubernetes/) |
| Metrics rules | Generated Prometheus recording rules and alert rules. | [Generated Prometheus rules](../../generated/prometheus/) and [PrometheusRule manifests](../../generated/prometheusrules/) |
| Log collection | Grafana Alloy collects OpenBao operational logs, audit logs, and platform logs. | [Docker Compose Alloy config](../../examples/docker-compose/alloy/config.alloy) |
| Log backend | Loki stores operational and audit exploration streams for dashboards and log alerts. | [Generated Loki alert artifacts](../../generated/loki/) |
| Dashboards | Grafana loads generated dashboard JSON files. | [Generated Grafana dashboards](../../generated/grafana/) |
| Response | Alerts link to runbooks under `docs/runbooks/`. | [Alert runbooks](../README.md#respond) |

## Local topology

The Docker Compose profile runs every component needed to validate dashboards,
alerts, log streams, and fixture scenarios on a workstation.

```text
OpenBao node 0
OpenBao node 1
OpenBao node 2
  | metrics
  v
Prometheus -> Grafana
  ^
  | dashboards and rules

OpenBao operational logs -> Alloy -> Loki -> Grafana
OpenBao audit logs       -> Alloy -> Loki -> Grafana
PostgreSQL               -> OpenBao database secrets fixture
```

## Included artifacts

| Artifact | Purpose |
| -------- | ------- |
| `generated/prometheus/` | Native Prometheus recording rules and alert rules. |
| `generated/prometheusrules/` | Prometheus Operator `PrometheusRule` manifests. |
| `generated/loki/` | Loki alert reference artifacts. |
| `generated/grafana/` | Grafana dashboard JSON files generated from dashboard contracts. |
| `examples/docker-compose/` | Local profile with OpenBao, PostgreSQL, Prometheus, Loki, Alloy, and Grafana. |
| `examples/kubernetes/` | Secure active-node and private all-node metrics scrape examples. |
| `contracts/` | Source contracts for generated metrics, streams, alerts, and dashboards. |

Generated artifacts are outputs. Edit contracts first, then regenerate.

```shell
make generate
```

## Local profile

Use the local profile for evaluation, screenshots, fixture scenarios, and live
query validation.

```shell
make fixtures-openbao
make generate
make compose-up
```

Open Grafana at `http://127.0.0.1:13000` and use the generated dashboards in
the `OpenBao` folder.

The local profile intentionally uses local credentials, HTTP endpoints, and
demo OpenBao setup. It is not a production deployment profile.

## Kubernetes adoption path

Use the Kubernetes examples as starting points, then adapt selectors, TLS,
secrets, labels, network policy, and Prometheus Operator selection labels to
your cluster.

1. Start with [Secure metrics scrape](../metrics/secure-metrics-scrape.md) for
   authenticated active-node metrics.
2. Add [All-node metrics scrape](../metrics/all-node-metrics-scrape.md) only
   when you need standby, sealed-node, or per-node Raft visibility.
3. Deploy generated Prometheus rules through your Prometheus Operator or
   metrics platform pipeline.
4. Deploy generated Grafana dashboards through file provisioning, Terraform,
   Grafana API automation, or your existing dashboard delivery workflow.
5. Configure Alloy or an equivalent collector to preserve OpenBao stream
   separation.
6. Send audit logs to a restricted exploration backend and to your approved
   audit archive path. Use
   [Audit archive reference design](../audit/audit-archive-reference-design.md)
   before you choose that path.

## Production adaptation checklist

Before you use this profile in a production environment, replace the local demo
assumptions with your platform controls.

- Use TLS for OpenBao, Prometheus, Loki, Grafana, Alloy, and remote writes.
- Store tokens, certificates, and backend credentials in your approved secret
  system.
- Rotate the OpenBao metrics token and collector credentials.
- Restrict all-node metrics listeners to the metrics collector path.
- Keep audit logs out of broad operational log tenants.
- Send audit logs to an approved archive outside short-term Loki exploration.
- Apply Grafana folder and data source permissions for audit dashboards.
- Set retention separately for metrics, operational logs, audit exploration,
  and audit archive.
- Review label cardinality before adding platform, tenant, namespace, or mount
  dimensions.
- Validate alerts against staging failure modes before paging production teams.

## Validate the profile

Run static and generated-artifact validation first.

```shell
make contracts-verify
make docs-verify
make validate-generated
make test-unit
```

Validate dashboard queries against the running local profile.

```shell
make compose-up
make validate-dashboard-queries
```

Run the full repository verification before you publish generated artifacts.

```shell
make verify
```

## Platform substitutions

You can replace one layer of the profile without replacing the architecture.

| Replace | Preserve |
| ------- | -------- |
| Prometheus with another metrics backend. | Metric intent, source prefix handling, alert semantics, and low-cardinality dimensions. |
| Loki with another log backend. | Stream separation, forbidden label policy, audit restrictions, and query-time investigation fields. |
| Grafana with another dashboard tool. | Dashboard questions, alert context, and restricted audit investigation views. |
| Alloy with another collector. | Source separation, least privilege, delivery health, and audit archive delivery. |
| PrometheusRule with another alerting engine. | Alert names, severities, runbook links, and response expectations. |

## What's next

- Use [Run the Docker Compose stack](../docker-compose.md) to start the local
  profile.
- Use [Implementation profiles](./README.md) to compare this profile with other
  adoption paths.
- Use [Adopt the reference architecture](../reference-architecture/adoption.md)
  when you need to port this profile to another observability platform.
- Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
  before you change metric prefixes or recording-rule generation.
- Use [Loki label strategy for OpenBao](../logging/loki-label-strategy.md)
  before you change log labels.
