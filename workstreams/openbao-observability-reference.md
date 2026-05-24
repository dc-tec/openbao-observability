# OpenBao Observability Reference Architecture

## 1. Executive summary

The **OpenBao Observability Reference Architecture** is a spec-first design for operating OpenBao with reliable metrics, dashboards, operational logs, audit logs, alerts, runbooks, deployment profiles, fixtures, and validation tests. It is not a dashboard pack. The source of truth should be a set of signal contracts that generate Grafana dashboards, PrometheusRule objects, Loki/Grafana log alerts, documentation tables, and test fixtures.

The architecture is OpenBao-native. Current OpenBao documentation still describes the default telemetry prefix as `vault`, exposes Prometheus metrics through `/v1/sys/metrics` with `format=prometheus`, and documents OpenBao metrics such as `vault.core.active`, `vault.core.unsealed`, `vault.audit.log_request_failure`, `vault.autopilot.healthy`, `vault.raft.peers`, `vault.token.count`, and `vault.expire.num_leases`. The reference must therefore support both historical `vault_*` Prometheus names and an explicitly configured `openbao_*` prefix without pretending that the ecosystem has already migrated. ([openbao.org][1])

Audit logs are treated as a security stream, not normal application logs. OpenBao audit devices record request and response objects as JSON lines, HMAC most string values by default, can block OpenBao when no enabled audit device can write, and have important limitations: not all paths are audited, non-string values may be plaintext, and `log_raw=true` disables hashing of sensitive material. ([openbao.org][2])

Grafana, Prometheus-compatible metrics, Loki, and Grafana Alloy are the reference implementation, but not the architecture boundary. Promtail is no longer an appropriate default for new designs because Grafana documents it as EOL as of March 2, 2026, with future feature development in Grafana Alloy. ([Grafana Labs][3])

Existing OpenBao and Vault dashboards are prior art only. A Grafana.com OpenBao dashboard explicitly says it is heavily influenced by a popular HashiCorp Vault dashboard, and an OpenBao Helm issue reports that generated dashboard labels did not match scraped Prometheus labels. That reinforces the design decision that this project should be contract-generated and fixture-tested, not copied from dashboard JSON. ([Grafana Labs][4])

Status language used in this document:

| Status                                  | Meaning                                                                                                             |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Confirmed OpenBao docs behavior**     | Directly supported by current OpenBao documentation.                                                                |
| **Observed OpenBao behavior**           | Observed in local OpenBao v2.5.2 and/or containerized OpenBao v2.5.4 during this review. Broader version/profile fixture coverage is still required. |
| **Grafana/Loki best practice**          | Directly supported by current Grafana/Loki/Alloy documentation.                                                     |
| **Kubernetes observability convention** | Based on kube-state-metrics, Kubernetes metrics, Prometheus Operator, or Kubernetes event collection behavior.      |
| **Recommendation / design decision**    | Architectural recommendation derived from the documented behavior.                                                  |
| **To validate**                         | Must be confirmed against live OpenBao metrics, logs, audit samples, chart rendering, or backend-specific behavior. |

## 2. Design goals

| Goal                                  | Design implication                                                                                                                                                                            |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| OpenBao-native observability          | Start from OpenBao documentation and live OpenBao fixtures. Do not assume Vault dashboards, Vault chart labels, or Vault operational guidance are identical.                                  |
| Safe audit-log handling               | Treat audit data as restricted security evidence. Separate `openbao.audit` and `openbao.audit_archive` from ordinary logs.                                                                    |
| Fast operational triage               | The overview dashboard should answer reachability, seal state, active leadership, HA/Raft health, audit health, latency, token/lease pressure, and runtime/platform pressure within one page. |
| Security-conscious metrics exposure   | Prefer authenticated active-node scraping for the baseline profile. Use all-node metrics only with a dedicated private metrics listener or equivalent network controls.                       |
| Low-cardinality dashboards and labels | Keep Prometheus grouping and Loki labels bounded. Avoid grouping by secret path, request path, policy, token accessor, entity ID, client IP, mount path, or request ID in shared dashboards.  |
| Kubernetes and VM/systemd support     | Provide Kubernetes-first examples, plus systemd/journald/file/logrotate patterns.                                                                                                             |
| Generated and testable artifacts      | Maintain signal contracts, then generate dashboards, rules, docs tables, and tests. Generated JSON should not be the primary source of truth.                                                 |
| Backend portability                   | Keep contracts backend-conscious: Prometheus/Mimir/Thanos/VictoriaMetrics for metrics; Loki/OpenSearch/Splunk/Elastic/SIEM/object storage for logs and audit archives.                        |
| Clear stream separation               | Separate metrics, operational logs, completed request logs, audit logs, audit archives, platform signals, and alerting.                                                                       |
| Explicit caveats                      | Document cardinality risks, slow gauge updates, standby scrape behavior, missing metrics by feature/version, audit blocking behavior, and dashboard limitations.                              |

## 3. Non-goals

This project does **not** replace a SIEM, compliance archive, immutable evidence store, or organization-specific retention policy. Loki can be the reference exploration backend, but it should not be presented as the final compliance-grade audit archive by default.

It does **not** expose sensitive audit metadata broadly. Audit logs may still reveal sensitive metadata even with HMAC enabled because OpenBao documents exceptions and plaintext behavior for non-string JSON values. ([openbao.org][2])

It does **not** fork existing Grafana dashboards as the source of truth. Prior art can be inspected for operator questions and panel layout ideas, but contracts and fixtures own the implementation.

It does **not** put every possible OpenBao metric on the overview dashboard. Feature-specific metrics belong in feature dashboards.

It does **not** enable debug/trace logging, runtime logger overrides, completed request logging, unauthenticated metrics, pprof, or in-flight request endpoints by default. Completed request logging is off by default in OpenBao and should remain a temporary troubleshooting stream. ([openbao.org][5])

It does **not** assume Kubernetes. Kubernetes is the first-class reference path, but VM/systemd deployments must be supported.

It does **not** assume Loki is the only log backend or that Grafana alerting is the only alerting engine.

## 4. Reference architecture

### 4.1 Logical architecture

```text
Metrics path
============

OpenBao servers
  -> GET /v1/sys/metrics
       params: format=prometheus
       auth: metrics token for secure baseline
       or private metrics-only listener for all-node profile
  -> Prometheus / Mimir / Thanos / VictoriaMetrics
  -> Grafana dashboards
  -> Alertmanager and/or Grafana-managed alerting
```

OpenBao documents that Prometheus scraping should use `metrics_path: "/v1/sys/metrics"` and `params: format: ["prometheus"]`, and that an OpenBao token with `read` and `list` capability is required for authenticated access. It also documents that the endpoint is only accessible on active nodes by default, with standby access enabled through unauthenticated metrics access. ([openbao.org][1])

```text
Operational logs path
=====================

OpenBao server logs
  -> stdout / journald / file
  -> Grafana Alloy or equivalent collector
  -> Loki / OpenSearch / Splunk / Elastic / other log backend
  -> Grafana operational log dashboards and log alerts
```

```text
Audit logs path
===============

OpenBao audit devices
  -> file audit device (file path or stdout), HTTP(S), syslog, socket
  -> restricted collector / security log shipper
  -> Loki for short-term exploration and correlation
  -> immutable archive / SIEM / object storage for security retention
  -> security dashboards, investigation views, audit health alerts
```

OpenBao recommends multiple audit devices because audit failures can prevent OpenBao from servicing requests; it also states that, with multiple devices, operators need the aggregate/union of device logs to build a complete picture. ([openbao.org][2])

```text
Platform path
=============

Kubernetes:
  Kubernetes events, pod logs, kube-state-metrics, kubelet/cAdvisor,
  node exporter, PVC metrics
    -> Prometheus/Loki
    -> Grafana OpenBao Kubernetes Platform dashboard

VM/systemd:
  journald, system logs, node exporter, disk metrics, audit file state
    -> Prometheus/Loki or equivalent
    -> Grafana VM/systemd dashboard
```

kube-state-metrics exposes Kubernetes object-state metrics such as container readiness and restart counters, while Kubernetes components expose Prometheus-format metrics through metrics endpoints. ([GitHub][6])

### 4.2 Kubernetes reference variant

```text
OpenBao StatefulSet
  ├─ API listener :8200
  ├─ optional private metrics-only listener :9101
  ├─ data PVC
  ├─ optional audit PVC mounted at /openbao/audit
  └─ audit file/stdout device

Kubernetes Services
  ├─ active service        -> secure baseline ServiceMonitor
  ├─ standby/headless svc  -> all-node profile, if configured
  └─ optional metrics svc  -> private NetworkPolicy-only scraping

Prometheus Operator / Alloy / Prometheus
  ├─ ServiceMonitor active-only
  ├─ PodMonitor/headless scrape for all-node profile
  └─ PrometheusRule

Alloy DaemonSet or sidecar
  ├─ pod logs
  ├─ Kubernetes events
  ├─ audit file tailing
  └─ optional remote write / Loki write
```

The OpenBao Helm chart is the documented Kubernetes installation method. The current chart source on `main` is chart `0.28.3` with appVersion `v2.5.4`; the release notes say this chart bumped OpenBao to `v2.5.4`. Its values still include active and standby services plus an optional audit PVC mounted at `/openbao/audit`. The chart’s ServiceMonitor template selects `openbao-active: "true"` in HA mode and scrapes `/v1/sys/metrics` with `format=prometheus`, so it should be treated as an **active-only prior-art profile**, not an all-node observability design. ([openbao.org][7], [GitHub][28], [GitHub][29], [GitHub][32], [GitHub][34])

### 4.3 VM/systemd reference variant

```text
openbao.service
  ├─ OpenBao HCL config
  ├─ API listener :8200
  ├─ optional localhost/private metrics listener :9101
  ├─ operational logs -> journald or log_file
  └─ audit file -> /var/log/openbao/audit.log

Host collectors
  ├─ Prometheus scrape or Alloy prometheus.scrape
  ├─ node_exporter
  ├─ Alloy loki.source.journal or loki.source.file
  └─ logrotate with postrotate SIGHUP for audit file reopen
```

The file audit device appends to a file, does not manage log rotation, and requires SIGHUP after rotation so OpenBao closes and reopens the file. ([openbao.org][8])

## 5. Metrics layer

### 5.1 OpenBao telemetry configuration

Baseline telemetry configuration:

```hcl
telemetry {
  prometheus_retention_time = "30s"
  disable_hostname          = true

  # Contract input. Keep "vault" for compatibility unless intentionally changed.
  metrics_prefix = "vault"

  # Keep defaults unless a profile explicitly changes them.
  usage_gauge_period      = "10m"
  maximum_gauge_cardinality = 500
}
```

OpenBao documents `metrics_prefix` with default `"vault"`, `usage_gauge_period` default `"10m"` for high-cardinality usage gauges, `maximum_gauge_cardinality` default `500`, `prometheus_retention_time` default `"24h"`, and `disable_hostname` as recommended with Prometheus to avoid hostname-prefixed metrics. ([openbao.org][1])

Recommended production hardening when a dedicated metrics listener is available:

```hcl
listener "tcp" {
  address         = "0.0.0.0:8200"
  cluster_address = "0.0.0.0:8201"

  tls_cert_file = "/etc/openbao/tls/tls.crt"
  tls_key_file  = "/etc/openbao/tls/tls.key"

  telemetry {
    disallow_metrics = true
  }
}

listener "tcp" {
  address = "0.0.0.0:9101"

  tls_cert_file = "/etc/openbao/tls/tls.crt"
  tls_key_file  = "/etc/openbao/tls/tls.key"

  telemetry {
    metrics_only                   = true
    unauthenticated_metrics_access = true
  }
}
```

This second profile is not the default. OpenBao documents `disallow_metrics`, `metrics_only`, `metrics_path`, and `unauthenticated_metrics_access`; it also shows a dedicated metrics-only listener pattern. Because standby metrics require unauthenticated metrics access, this profile must be isolated with NetworkPolicy, private routing, mTLS/sidecar controls, or host firewall rules. ([openbao.org][9])

### 5.2 Prometheus endpoint and scrape config

Prometheus scrape sketch:

```yaml
scrape_configs:
  - job_name: openbao
    metrics_path: /v1/sys/metrics
    params:
      format: [prometheus]
    scheme: https
    bearer_token_file: /etc/prometheus/secrets/openbao-metrics-token/token
    tls_config:
      ca_file: /etc/prometheus/secrets/openbao-ca/ca.crt
    static_configs:
      - targets:
          - openbao-active.openbao.svc:8200
```

Metrics token policy sketch:

```hcl
path "sys/metrics" {
  capabilities = ["read", "list"]
}
```

For Prometheus Operator, provide both `ServiceMonitor` and `PodMonitor` examples. The active-only `ServiceMonitor` should target the active service. The all-node `PodMonitor` or headless-service `ServiceMonitor` should be a separate profile, clearly labeled as requiring private metrics-only access or an equivalent local scrape design. Prometheus Operator resources such as `ServiceMonitor`, `PodMonitor`, and `PrometheusRule` are first-class Kubernetes CRDs referenced by the operator API. ([Prometheus Operator][10])

### 5.3 Scrape profiles

| Profile                   | Target                                                                                             |                                                                                  Auth | Purpose                                                                | Security posture                                                                                                        | Caveats                                                                                                                    |
| ------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------: | ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| Secure baseline           | Active OpenBao endpoint or active service                                                          |                                                                         Metrics token | Safe default dashboards and critical alerts                            | No unauthenticated metrics; TLS; token scoped to `sys/metrics`; scrape path restricted                                  | Standby node state is mostly invisible; active-only queries cannot prove all pods are healthy.                             |
| Full node visibility      | All OpenBao pods/nodes through private metrics-only listener, local sidecar, or controlled network | Usually unauthenticated listener isolated by network, or local scrape-to-remote-write | Leadership, sealed standby, runtime, Raft follower, pod-level pressure | Metrics listener must not be generally reachable; use NetworkPolicy, SG/firewall, mTLS proxy, or node-local permissions | OpenBao docs say standby metrics require unauthenticated access; this needs explicit security sign-off. ([openbao.org][1]) |
| BYO Prometheus-compatible | Existing Prometheus/Mimir/Thanos/VictoriaMetrics                                                   |                                                                      Operator-defined | Reuse existing platform                                                | Provide scrape contract, recording rules, labels, and cardinality guardrails                                            | Must map labels and metric prefix into normalized recording rules.                                                         |
| VM/systemd                | Localhost/private listener                                                                         |                                                  Token or local metrics-only listener | Non-Kubernetes operations                                              | Host firewall; systemd unit permissions; TLS where remote                                                               | Local collectors should not expose metrics externally by accident.                                                         |

### 5.4 Metric prefix strategy

OpenBao’s docs still use `vault.*` metric names and default `metrics_prefix = "vault"`. Prometheus format converts metric namespaces into Prometheus-compatible names, as shown by the documented `/sys/metrics` endpoint returning names such as `vault_audit_log_request` and `vault_audit_log_request_failure`. ([openbao.org][11])

Contract rules:

| Rule | Decision |
| ---- | -------- |
| Source docs metric name | Always store the official docs name, for example `vault.core.active`. |
| Prometheus metric name | Generated from `metricPrefix`, for example `${p}_core_active`, where `${p}` is `vault` or `openbao`. |
| Dashboard generation | Generate direct-prefix dashboards for performance: `vault_*` by default, `openbao_*` when explicitly configured. |
| Compatibility mode | Optional discovery query mode using `{__name__=~"^(vault\|openbao)_core_active$"}` during migration windows. |
| Alerting | Prefer generated single-prefix rules or normalized recording rules. Avoid expensive regex queries in critical alerts unless migration requires them. |
| Recording rules | Produce canonical internal names such as `openbao:core_active:max` regardless of source prefix. |
| Filters | If OpenBao `prefix_filter` is used, validate filters under the configured prefix. Do not hardcode `vault.` filters in an `openbao`-prefixed deployment. |
| Validation | Capture `/v1/sys/metrics?format=prometheus` fixtures for every supported OpenBao version and prefix. |

Observed OpenBao v2.5.2 and v2.5.4 behavior: `metrics_prefix = "openbao"` produced expected names such as `openbao_core_active`, `openbao_core_handle_request`, `openbao_audit_log_request_failure`, `openbao_expire_num_leases`, and `openbao_runtime_num_goroutines`. This answers the basic prefix question, but not the full matrix of feature-specific metrics, labels, and timer sanitization.

### 5.5 High-cardinality metric concerns

OpenBao usage and lifecycle metrics can include labels such as namespace, auth method, mount point, policy, TTL, token type, secret engine, and cluster. Token count by policy, secret count by mount, and lease creation by mount/TTL can be operationally useful but should not be promoted into fleet-wide overview panels without cardinality limits and access control. OpenBao documents token count by policy, token creation labels, secret KV count by mount point, and secret lease creation labels. ([openbao.org][12])

Current project validation scope: captured fixtures cover root-namespace behavior, one `team-a` child namespace for userpass, AppRole, token, KV, transit, PKI issue/revoke/failure, policy, and audit activity, and a Raft HA topology with three voters plus one non-voter read replica. OpenBao documents namespace commands, namespace limits, namespace-related lease metric labeling, Raft non-voters, and non-voter/read-scalability behavior, but this project should mark nested namespace behavior, database lease behavior inside namespaces, operator-managed read-replica behavior, and production read-capacity thresholds **to validate** until dedicated fixtures or live validation exist. The namespace PKI fixture also shows namespace-derived metric-family names shaped like `<prefix>_<sanitized_namespace>_pki_issue`, so namespace dashboards should not assume labels are the only namespace dimension. ([openbao.org][1], [openbao.org][35], [openbao.org][36], [openbao.org][37], [openbao.org][38])

Recommended contract defaults:

| Metric family                       | Overview? | Detailed dashboard? | Label handling                                                          |
| ----------------------------------- | --------: | ------------------: | ----------------------------------------------------------------------- |
| `vault.core.*`                      |       Yes |                 Yes | `cluster`, `instance`, `namespace` where bounded.                       |
| `vault.audit.*`                     |       Yes |                 Yes | Device-specific metrics only when device names are bounded.             |
| `vault.autopilot.*`, `vault.raft.*` |       Yes |                 Yes | Peer/node labels allowed in HA/Raft dashboard, not all overview panels. |
| `vault.token.count`                 |       Yes |                 Yes | No policy grouping in overview.                                         |
| `vault.token.count.by_auth`         |   Limited |                 Yes | Auth method usually bounded; still restricted.                          |
| `vault.token.count.by_policy`       |        No |     Yes, restricted | Policy names can be sensitive and unbounded.                            |
| `vault.secret.kv.count`             |        No |     Yes, restricted | Mount path can be sensitive.                                            |
| `vault.secret.lease.creation`       |        No |     Yes, restricted | Secret engine, mount, TTL can create high cardinality.                  |
| Route/path/request-level logs       | No labels |  Investigation only | Never labels. Query-time parsing only.                                  |

### 5.6 Slow-updating metrics

Do not alert on slow usage gauges with narrow windows. OpenBao documents token count as updated every 10 minutes and identity alias count as updated every `usage_gauge_period`; lease bucket metrics are governed by `lease_metrics_epsilon` and `num_lease_metrics_buckets`. ([openbao.org][12])

Recommended handling:

```promql
max_over_time(${p}_token_count[30m])
max_over_time(${p}_identity_num_entities[30m])
max_over_time(${p}_expire_num_leases[30m])
```

### 5.7 Metric validation workflow

Every release should include:

1. A metric fixture captured from a local OpenBao dev server.
2. A metric fixture from a three-node Raft cluster.
3. A metric fixture with at least one audit device enabled.
4. A metric fixture with `metrics_prefix = "openbao"`.
5. A generated matrix of contract metric names, observed Prometheus names, labels, units, and missing metrics.

Do not merge a panel or alert as “confirmed” until its metric name, labels, type, and unit have been observed in fixtures for at least one supported OpenBao version.

Observed OpenBao v2.5.2 and v2.5.4 caveat: raw Prometheus label sets are not uniform. `*_core_active` had a `cluster` label, `*_core_unsealed` emitted both `cluster=""` and a real cluster value, while `*_expire_num_leases` and runtime gauges were unlabeled in a dev profile. Generated recording rules should normalize labels from scrape metadata and should not assume every raw OpenBao metric has a usable `cluster` label.

Observed OpenBao v2.5.4 HA/Raft caveat: `vault_raft_peers` was present in the
captured HA/Raft fixture on the active node, but absent from the current live
Docker Compose all-node scrape. The live stack still exposed
`vault_raft_storage_stats_commit_index{peer_id=...}` on every Raft node.
Normalized peer-count recording rules should therefore prefer `*_raft_peers`
when present and fall back to counting `*_raft_storage_stats_commit_index` by
`peer_id` in all-node scrape profiles.

## 6. Overview dashboard contract

### 6.1 Contract metadata

```yaml
contract_id: openbao-overview
title: OpenBao Overview
version: v0.1
status: draft
datasources:
  metrics: prometheus-compatible
  logs: loki-compatible
  platform: prometheus-compatible
variables:
  - datasource
  - cluster
  - environment
  - namespace
  - job
  - metric_prefix   # vault | openbao
  - scrape_profile  # active-only | all-node
  - log_datasource
security_classification: internal-restricted
```

PromQL shorthand used below:

```text
${p} = selected metric prefix, usually vault

avg5(${metric}) =
  sum(rate(${metric}_sum{cluster=~"$cluster"}[5m]))
/
  sum(rate(${metric}_count{cluster=~"$cluster"}[5m]))

p99(${metric}) =
  max by (cluster, instance) (${metric}{cluster=~"$cluster", quantile="0.99"})
```

Summary metrics in OpenBao are documented in milliseconds, but generated dashboards must validate observed Prometheus units against fixtures before applying unit conversions. Metrics with uppercase or hyphenated docs names, such as `vault.raft.leader.lastContact` or `vault.raft-storage.*`, are marked **to validate** for exact Prometheus sanitization.

Raw query sketches below are intentionally illustrative. Production dashboards should use generated recording rules that normalize `job`, `instance`, Kubernetes labels, and OpenBao cluster labels because observed raw metric labels differ by metric family and deployment profile.

### 6.2 `openbao-overview` panel contract

OpenBao metrics in groups A through G are based on the current OpenBao all-metrics documentation for core, audit, Autopilot, Raft, lease, token, runtime, barrier, cache, and WAL metrics. ([openbao.org][12])

| Row / section | Panel name | Purpose | Docs metric name | Prometheus metric name | PromQL | Healthy state | Alert candidate | Caveats |
| ------------- | ---------- | ------- | ---------------- | ---------------------- | ------ | ------------- | --------------- | ------- |
| A. Cluster status | Scrape health | Is OpenBao reachable by Prometheus? | N/A | `up` | `min by (cluster) (up{job=~"$job",cluster=~"$cluster"})` | `1` for expected targets | Yes | Active-only profile only proves active endpoint reachability. |
| A | Active node count | Is there exactly one active node? | `vault.core.active` | `${p}_core_active` | `sum by (cluster) (max by(cluster,instance) (${p}_core_active{cluster=~"$cluster"}))` | `1` | Yes | All-node profile preferred. Active-only scrape can hide split-brain/stale target issues. |
| A | Unsealed node count | How many scraped nodes are unsealed? | `vault.core.unsealed` | `${p}_core_unsealed` | `sum by (cluster) (${p}_core_unsealed{cluster=~"$cluster",cluster!=""})` | Equals expected node count in all-node profile; `1` active-only | Yes | Observed v2.5.2 and v2.5.4 emitted an extra `cluster=""` series with value `0`; generated rules must exclude or normalize it. This is node count, not voter count. |
| A | Leadership setup failures | Detect failure to become leader | `vault.core.leadership_setup_failed` | `${p}_core_leadership_setup_failed_count` | `sum by(cluster) (increase(${p}_core_leadership_setup_failed_count{cluster=~"$cluster"}[15m]))` | `0` | Yes | Summary `_count` counts observed failure timing samples; verify exact suffix in fixtures. |
| A | Leadership lost / churn | Detect leadership instability | `vault.core.leadership_lost`, `vault.raft.state.leader` | `${p}_core_leadership_lost_count`, `${p}_raft_state_leader` | `sum by(cluster) (increase(${p}_core_leadership_lost_count{cluster=~"$cluster"}[30m]))` | No unexpected increases | Yes | Combine with Raft transition counters for context. |
| B. Request health | In-flight requests | Is request concurrency rising? | `vault.core.in_flight_requests` | `${p}_core_in_flight_requests` | `sum by(cluster) (${p}_core_in_flight_requests{cluster=~"$cluster"})` | Stable baseline | Yes | Threshold must be workload-specific. |
| B | Non-login request latency | User/API path latency | `vault.core.handle_request` | `${p}_core_handle_request` | `avg5(${p}_core_handle_request)` and `p99(${p}_core_handle_request)` | Stable baseline | Yes | Summary quantiles cannot be aggregated across instances safely. |
| B | Login request latency | Auth path latency | `vault.core.handle_login_request` | `${p}_core_handle_login_request` | `avg5(${p}_core_handle_login_request)` | Stable baseline | Yes | Auth backend-specific dashboards should break this down later. |
| B | Token check latency | Token validation pressure | `vault.core.check_token` | `${p}_core_check_token` | `avg5(${p}_core_check_token)` | Stable baseline | Warning | Can rise with storage/cache pressure. |
| B | Request rate | Traffic context for latency | `vault.core.handle_request` | `${p}_core_handle_request_count` | `sum by(cluster) (rate(${p}_core_handle_request_count{cluster=~"$cluster"}[5m]))` | Informational | No | Not an error metric. |
| C. Audit health | Audit request failures | Detect audit request write failures | `vault.audit.log_request_failure` | `${p}_audit_log_request_failure` | `sum by(cluster) (increase(${p}_audit_log_request_failure{cluster=~"$cluster"}[5m]))` | `0` | Critical | OpenBao docs treat non-zero as crucial. |
| C | Audit response failures | Detect audit response write failures | `vault.audit.log_response_failure` | `${p}_audit_log_response_failure` | `sum by(cluster) (increase(${p}_audit_log_response_failure{cluster=~"$cluster"}[5m]))` | `0` | Critical | Can indicate one configured device failed. |
| C | Audit request latency | Audit path saturation | `vault.audit.log_request` | `${p}_audit_log_request` | `avg5(${p}_audit_log_request)` | Stable, low | Warning | High latency can become request latency. |
| C | Audit response latency | Audit response path saturation | `vault.audit.log_response` | `${p}_audit_log_response` | `avg5(${p}_audit_log_response)` | Stable, low | Warning | Device-specific drilldown required. |
| C | Per-device audit latency | Identify slow audit device | `vault.audit.{DEVICE}.log_request`, `vault.audit.{DEVICE}.log_response` | `${p}_audit_<device>__log_request`, `${p}_audit_<device>__log_response` | `{__name__=~"^${p}_audit_.+__log_(request\|response)$"}` | Stable, low | No by default | Observed `local-file` path exported as `vault_audit_local_file__log_request` / `openbao_audit_local_file__log_request`; aggregate failure metrics exist, but device-specific failure counters were not observed. |
| D. HA/Raft quick view | Autopilot healthy | Is Raft Autopilot healthy? | `vault.autopilot.healthy` | `${p}_autopilot_healthy` | `min by(cluster) (${p}_autopilot_healthy{cluster=~"$cluster"})` | `1` | Critical | Autopilot metrics run on active node. |
| D | Failure tolerance | Healthy nodes above quorum | `vault.autopilot.failure_tolerance` | `${p}_autopilot_failure_tolerance` | `min by(cluster) (${p}_autopilot_failure_tolerance{cluster=~"$cluster"})` | `>= 1` for HA clusters | Critical | Single-node/dev clusters are expected `0`. |
| D | Raft peers | Configured Raft peer count | `vault.raft.peers` | `${p}_raft_peers` | `max by(cluster) (${p}_raft_peers{cluster=~"$cluster"})` | Expected peer count | Warning | Not a quorum-health metric alone. Observed `4` in a fixture with three voters plus one non-voter read replica. |
| D | Leader last contact | Follower contact health | `vault.raft.leader.lastContact` | `${p}_raft_leader_lastContact` | `p99(${p}_raft_leader_lastContact)` | Stable, low | Warning | **To validate** Prometheus name case. |
| D | Candidate transitions | Election churn | `vault.raft.state.candidate` | `${p}_raft_state_candidate` | `sum by(cluster) (increase(${p}_raft_state_candidate{cluster=~"$cluster"}[30m]))` | `0` outside failover | Warning | Expected during planned restart. |
| D | Leader transitions | Leadership churn | `vault.raft.state.leader` | `${p}_raft_state_leader` | `sum by(cluster) (increase(${p}_raft_state_leader{cluster=~"$cluster"}[30m]))` | Stable | Warning | Use with maintenance windows. |
| D | Snapshot / replication latency | Replication pressure | `vault.raft.replication.*`, `vault.raft.snapshot.*` | `${p}_raft_replication_*`, `${p}_raft_snapshot_*` | `avg5(${p}_raft_replication_heartbeat)` | Stable baseline | No in overview | **To validate** exact timer export names. |
| E. Token and lease pressure | Token count | Token-store size | `vault.token.count` | `${p}_token_count` | `max_over_time(${p}_token_count[30m])` | Stable baseline | Warning | Updates every 10 minutes; label set must be fixture-tested before adding `cluster` grouping. |
| E | Token creation rate | Token churn | `vault.token.creation` | `${p}_token_creation` | `sum by(cluster) (rate(${p}_token_creation{cluster=~"$cluster"}[15m]))` | Stable baseline | Warning | Labels include mount/TTL/token type; avoid over-grouping. |
| E | Token count by auth method | Auth-source pressure | `vault.token.count.by_auth` | `${p}_token_count_by_auth` | `topk(10, sum by(auth_method) (max_over_time(${p}_token_count_by_auth[30m])))` | Expected auth methods | No by default | Auth method is lower-cardinality than policy but still restricted; label set must be fixture-tested. |
| E | Lease count | Lease-store size | `vault.expire.num_leases` | `${p}_expire_num_leases` | `max_over_time(${p}_expire_num_leases[30m])` | Stable baseline | Warning | Observed v2.5.2 and v2.5.4 dev samples had no `cluster` label; normalize via recording rule. |
| E | Irrevocable leases | Leases OpenBao cannot revoke automatically | `vault.expire.num_irrevocable_leases` | `${p}_expire_num_irrevocable_leases` | `${p}_expire_num_irrevocable_leases` | `0` or known baseline | Warning | Investigate by mount/engine outside overview; normalize labels via recording rule. |
| E | Lease expiration errors | Expiration failure signal | `vault.expire.lease_expiration.error` | `${p}_expire_lease_expiration_error` | `sum(increase(${p}_expire_lease_expiration_error[15m]))` | `0` | Warning | Verify exact Prometheus sanitization and label set. |
| F. Runtime health | Goroutines | Runtime pressure | `vault.runtime.num_goroutines` | `${p}_runtime_num_goroutines` | `max by(instance) (${p}_runtime_num_goroutines)` | Stable baseline | Warning | Observed v2.5.2 and v2.5.4 dev samples had no `cluster` label; combine with scrape labels. |
| F | Heap objects | Memory pressure | `vault.runtime.heap_objects` | `${p}_runtime_heap_objects` | `max by(instance) (${p}_runtime_heap_objects)` | Stable baseline | Warning | Use with heap/sys bytes. |
| F | Allocated/system bytes | Go memory footprint | `vault.runtime.sys_bytes` | `${p}_runtime_sys_bytes` | `max by(instance) (${p}_runtime_sys_bytes)` | Stable baseline | Warning | Compare with container memory. |
| F | GC pause | Runtime pause pressure | `vault.runtime.total_gc_pause_ns` | `${p}_runtime_total_gc_pause_ns` | `rate(${p}_runtime_total_gc_pause_ns[5m])` | Stable baseline | Warning | Observed as a gauge in v2.5.2 and v2.5.4 dev; final treatment needs fixture validation. |
| F | Process CPU/memory | Platform pressure | cAdvisor/node exporter | `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes` | `sum(rate(container_cpu_usage_seconds_total{pod=~"openbao.*"}[5m]))` | Below requests/limits | Warning | Kubernetes-specific. |
| G. Storage/barrier/cache | Barrier GET latency | Storage/barrier read latency | `vault.barrier.get` | `${p}_barrier_get` | `avg5(${p}_barrier_get)` | Stable baseline | Warning | High latency affects request paths. |
| G | Barrier PUT latency | Storage/barrier write latency | `vault.barrier.put` | `${p}_barrier_put` | `avg5(${p}_barrier_put)` | Stable baseline | Warning | Write-heavy workloads. |
| G | Barrier LIST latency | Storage/barrier list latency | `vault.barrier.list` | `${p}_barrier_list` | `avg5(${p}_barrier_list)` | Stable baseline | No | Can be impacted by list-heavy clients. |
| G | Cache hit ratio | Storage cache effectiveness | `vault.cache.hit`, `vault.cache.miss` | `${p}_cache_hit`, `${p}_cache_miss` | `sum(rate(${p}_cache_hit[5m])) / clamp_min(sum(rate(${p}_cache_hit[5m])) + sum(rate(${p}_cache_miss[5m])), 1)` | Stable baseline | No | Informational unless correlated with latency. |
| G | WAL pressure | WAL backlog/storage pressure | `vault.wal.*` | `${p}_wal_*` | `max(${p}_wal_gc_total)` | Stable baseline | Warning | Only relevant where WAL metrics exist. |
| H. Kubernetes/platform | Pod readiness | Are OpenBao pods ready? | kube-state-metrics | `kube_pod_container_status_ready` | `min by(cluster,namespace,pod) (kube_pod_container_status_ready{namespace=~"$namespace",pod=~"openbao.*"})` | `1` for expected pods | Critical | Use workload selectors in final rules. |
| H | Restarts | Crash/restart pressure | kube-state-metrics | `kube_pod_container_status_restarts_total` | `sum by(cluster,pod) (increase(kube_pod_container_status_restarts_total{pod=~"openbao.*"}[15m]))` | `0` unexpected | Warning | kube-state-metrics metric is stable. ([GitHub][6]) |
| H | CPU | CPU saturation | kubelet/cAdvisor | `container_cpu_usage_seconds_total` | `sum by(pod) (rate(container_cpu_usage_seconds_total{pod=~"openbao.*"}[5m]))` | Below request/limit baseline | Warning | Requires kubelet/cAdvisor collection. |
| H | Memory | Memory pressure/OOM risk | kubelet/cAdvisor | `container_memory_working_set_bytes` | `max by(pod) (container_memory_working_set_bytes{pod=~"openbao.*"})` | Below limit baseline | Warning | Pair with OOM events. |
| H | PVC/disk pressure | Audit/data storage risk | kubelet/kube-state-metrics | `kubelet_volume_stats_*` | `kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes` | Above threshold | Critical for audit PVC | Required for file audit profile. |
| H | Node pressure | Node scheduling/runtime risk | kube-state-metrics | `kube_node_status_condition` | `max by(node,condition) (kube_node_status_condition{condition=~"DiskPressure\|MemoryPressure",status="true"})` | `0` | Warning | Node labels must be bounded. |
| H | Kubernetes events | Recent platform causes | Alloy events stream | Loki stream | `{log_stream="platform.kubernetes"} \| json \| involvedObject_name=~"openbao.*"` | No warning/error spikes | No | Alloy can tail Kubernetes events. ([Grafana Labs][13]) |

## 7. Additional dashboard roadmap

| Dashboard                       | Purpose                               | Intended audience               | Key panels                                                                                                 | Required data sources                       | Security/cardinality concerns                                 | Alert/runbook relationship                              |
| ------------------------------- | ------------------------------------- | ------------------------------- | ---------------------------------------------------------------------------------------------------------- | ------------------------------------------- | ------------------------------------------------------------- | ------------------------------------------------------- |
| OpenBao Overview                | One-page triage                       | SRE, platform, security on-call | Reachability, seal, leader count, audit failures, latency, Raft, token/lease pressure, runtime, pod health | Prometheus, optional Loki, platform metrics | No paths, policies, entity IDs, client IPs, request IDs       | Entry point for most critical alerts.                   |
| OpenBao HA/Raft                 | Deep HA and integrated storage health | OpenBao operators               | Autopilot, failure tolerance, peers, leader contact, elections, follower lag, snapshots, Raft storage, WAL | Prometheus                                  | Node/peer labels allowed; no secret paths                     | Raft peer unhealthy, quorum risk, leadership churn.     |
| OpenBao Audit Overview          | Audit device health and volume        | Security, SRE                   | Audit failures, latency, event volume, request/response balance, missing stream                            | Prometheus, Loki                            | Restricted folder; no broad developer access                  | Audit failures, missing audit stream, archive degraded. |
| OpenBao Audit Investigation     | Security investigation workflow       | Security responders             | Request ID drilldown, sys mutations, auth activity, risky paths, device gaps                               | Loki, optional SIEM                         | Very restricted; query-time parsing only; no high-card labels | Links from audit alerts; not a general dashboard.       |
| OpenBao Operational Logs        | Server health from logs               | SRE                             | Error/warn rates, seal/unseal logs, Raft/storage warnings, debug level detection                           | Loki/log backend                            | Operational logs may contain metadata; restricted internal    | Debug logging, Raft warnings, collector issues.         |
| OpenBao Auth/Identity           | Auth and identity lifecycle           | IAM/security platform           | Login latency, auth activity, entity counts, alias counts, locked users                                    | Prometheus, audit logs                      | Entity IDs/usernames forbidden as labels                      | Elevated auth failures, identity growth.                |
| OpenBao Token/Lease Lifecycle   | Token/lease pressure                  | SRE/security                    | Token count, creation, TTL buckets, lease count, expiration errors, irrevocable leases                     | Prometheus                                  | Policy/mount grouping restricted                              | Token growth, lease expiration errors.                  |
| OpenBao Database Secrets        | Database dynamic credential lifecycle | OpenBao platform owners         | Credential create/renew/revoke rate, operation latency, failures, lease creation, audit streams            | Prometheus, audit logs                      | Database roles, lease IDs, and credential paths are sensitive | Database feature warnings, irrevocable leases.          |
| OpenBao Secret Engines / Mounts | Secret engine behavior                | OpenBao platform owners         | KV count, lease creation by engine, mount table size, engine errors                                        | Prometheus, audit logs                      | Mount paths may be sensitive                                  | Engine-specific runbooks.                               |
| OpenBao Transit                 | Transit usage and latency             | App platform, OpenBao owners    | Transit request rates/latency from audit/logs, errors                                                      | Audit logs, operational logs                | Key names/paths sensitive; no labels                          | To validate with real audit samples.                    |
| OpenBao PKI                     | PKI engine operations                 | PKI owners, security            | Tidy metrics, issuance/revocation activity, errors                                                         | Prometheus, audit logs                      | Issuer/cert paths sensitive                                   | PKI tidy failure, issuance anomaly.                     |
| OpenBao Runtime / Go Process    | Runtime pressure                      | SRE                             | Goroutines, heap, sys bytes, GC, CPU/mem                                                                   | Prometheus, platform metrics                | Low risk, but internal only                                   | Runtime pressure alerts.                                |
| OpenBao Kubernetes Platform     | Pod/node/storage context              | Platform engineers              | Readiness, restarts, CPU/mem, PVC, node pressure, events                                                   | Prometheus, Loki                            | Pod labels bounded; events may reveal metadata                | Pod unavailable, PVC pressure.                          |
| OpenBao SLO / Availability      | User-facing availability              | SRE leadership                  | Availability, latency, error budget, scrape health, request success proxies                                | Prometheus, synthetic probes, optional logs | SLO data should not expose paths                              | SLO burn alerts.                                        |

## 8. Logging layer

### 8.1 Stream contract

Grafana Alloy can collect Kubernetes pod logs, system logs, Kubernetes events, and file logs, and forward them to Loki; Alloy’s file source reads absolute file targets and forwards log entries to Loki components. ([Grafana Labs][14])

| Stream                       | Purpose                                                       |                  Default | Source                            | Format                  | Collector                   | Primary backend                | Access level           | Retention recommendation       | Security caveats                                                    |
| ---------------------------- | ------------------------------------------------------------- | -----------------------: | --------------------------------- | ----------------------- | --------------------------- | ------------------------------ | ---------------------- | ------------------------------ | ------------------------------------------------------------------- |
| `openbao.operational`        | Server health, startup, seal, storage, Raft, runtime messages |                  Enabled | stdout, journald, `log_file`      | JSON recommended        | Alloy                       | Loki or alternative            | SRE/platform           | 14–30 days                     | Internal metadata; not secret-safe by assumption.                   |
| `openbao.completed_requests` | Temporary request troubleshooting                             |                 Disabled | OpenBao completed request logging | Operational log format  | Alloy                       | Loki restricted                | SRE break-glass        | 24–72 hours                    | May expose paths/metadata; not audit substitute.                    |
| `openbao.audit`              | Security audit exploration                                    | Enabled in baseline/prod | Audit device file/stdout          | JSON lines              | Alloy or restricted shipper | Loki restricted                | Security + limited SRE | 7–30 days for exploration      | Sensitive even with HMAC; no broad access.                          |
| `openbao.audit_archive`      | Long-term evidence and compliance retention                   |       Production enabled | Audit file/security shipper/SIEM  | JSON or backend-native  | Security pipeline           | SIEM/object store/WORM archive | Security/compliance    | Org policy, often months/years | Tamper resistance, immutability, legal retention outside this repo. |
| `platform.kubernetes`        | Events, pod logs, kubelet context                             |             K8s profiles | Kubernetes API, pod log files     | JSON/logfmt             | Alloy                       | Loki                           | Platform/SRE           | 7–30 days                      | Events can reveal object names and failure details.                 |
| `platform.systemd`           | VM host/service context                                       |               VM profile | journald/syslog/files             | journald fields or JSON | Alloy                       | Loki/OpenSearch/etc.           | Platform/SRE           | 14–30 days                     | Host logs can include sensitive process args.                       |

### 8.2 Recommended OpenBao logging configuration

```hcl
log_level  = "info"
log_format = "json"

# VM/systemd option. Prefer journald/stdout in containers.
# log_file = "/var/log/openbao/openbao.log"

# Must remain disabled by default.
log_requests_level = "off"
```

OpenBao configuration supports `log_level`, `log_format`, `log_file`, `log_rotate_duration`, `log_rotate_bytes`, and `log_rotate_max_files`; `log_level` defaults to `info` and supports `trace`, `debug`, `info`, `warn`, and `error`. ([openbao.org][15])

Completed request logging is controlled by `log_requests_level`, defaults to `off`, supports `error`, `warn`, `info`, `debug`, `trace`, and `off`, and only emits if OpenBao’s `log_level` includes that level. It can be enabled/disabled with config plus SIGHUP. ([openbao.org][5])

Runtime logger changes through `/sys/loggers` are useful for short troubleshooting windows but are not persisted across reload/restart. The API supports modifying all loggers or a named logger, reading levels, and reverting levels. ([openbao.org][16])

## 9. Audit logging design

### 9.1 Principles

Audit logs are security records. They should be separated from operational logs at the collector, backend, Grafana folder, data source permission, retention policy, and runbook level.

OpenBao audit devices log request and response JSON objects. The documented `type` field is currently `request` or `response`, and request/response pairs can be matched by a unique request identifier. By default, sensitive information is hashed before logging. ([openbao.org][2])

Audit logs do not cover every endpoint. OpenBao documents non-audited paths including `sys/init`, `sys/seal-status`, `sys/seal`, `sys/unseal`, `sys/leader`, `sys/health`, `sys/storage/raft/bootstrap`, and `sys/storage/raft/join`; additionally, unauthenticated `sys/metrics`, `sys/pprof/*`, and `sys/in-flight-req` are not audited if listener settings allow unauthenticated access. This means seal/unseal and health-related detection must rely on metrics and operational logs, not audit logs alone. ([openbao.org][2])

`log_raw=false` is the reference default. OpenBao documents `log_raw` default `false` and states that enabling it logs security-sensitive information without hashing. `log_raw=true` should appear only in a break-glass test fixture designed to prove the linter rejects it. ([openbao.org][2])

### 9.2 HMAC behavior and limitations

OpenBao HMACs most strings in requests and responses using HMAC-SHA256 with a salt, and `/sys/audit-hash` can be used to compare known values. However, OpenBao documents that only strings from JSON or returned in JSON are HMAC’d; integers, booleans, and other non-string types may pass through in plaintext, and OpenBao has exceptions. Therefore, audit logs are still sensitive metadata and should not be broadly exposed. ([openbao.org][2])

### 9.3 Device choices

| Device              | Recommended use                                                            | Caveats                                                                                                                                                                        |
| ------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| File                | Baseline and production default                                            | Appends to file, no built-in rotation, SIGHUP required after rotation; arbitrary file-write risk if API audit creation is allowed. ([openbao.org][8])                          |
| File to stdout      | Demo/container baseline where file tailing is not configured               | `stdout` is a special `file_path` value for the file audit device. Easy collection, but mixes audit with container log pipeline unless relabeled; weaker separation.           |
| HTTP(S)             | Optional secondary path to an internal audit gateway after failure testing | No retry and fully synchronous by default; use HTTPS; arbitrary server connection risk. ([openbao.org][17])                                                                    |
| Syslog              | VM environments with mature local syslog and TCP forwarding                | No configurable destination; local agent only; UDP packet-size risk; use file plus syslog or syslog reading the file. ([openbao.org][18])                                      |
| Socket TCP/UDP/Unix | Specialized integrations                                                   | UDP can lose logs without OpenBao indication; TCP connection loss can omit a single entry, and unavailable TCP destinations can make OpenBao unresponsive. ([openbao.org][19]) |

### 9.4 Multiple audit devices and blocking behavior

OpenBao attempts to write to all configured audit devices. A request can complete if at least one audit device writes successfully, but a blocking audit device can hang requests until resolved. The reference should therefore test failure modes for every production profile: disk full, permission denied, collector down, HTTP audit endpoint slow/down, syslog down, and socket endpoint down. ([openbao.org][2])

### 9.5 Declarative audit configuration

OpenBao supports declarative `audit` stanzas in server configuration. Audit devices are created and removed on the active node during restarts and SIGHUP events; the same configuration should exist across all servers. The docs example uses `log_raw=true`, but this reference intentionally overrides that with `log_raw="false"`. OpenBao v2.5.2 and v2.5.4 rejected `bao audit enable file ...` in dev mode unless API audit creation is explicitly enabled with `unsafe_allow_api_audit_creation=true`, so baseline examples should prefer declarative audit configuration and reserve API audit creation for isolated unsafe tests. ([openbao.org][15], [openbao.org][20])

#### A. Kubernetes demo profile

```hcl
audit "file" "stdout" {
  description = "Demo-only audit stream to container stdout."

  options {
    file_path     = "stdout"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
```

Caveat: stdout is acceptable for local demo and kind environments. It is not the preferred production archive path.

#### B. Kubernetes baseline profile

```hcl
audit "file" "audit-file" {
  description = "Baseline audit file on mounted audit volume."

  options {
    file_path     = "/openbao/audit/audit.log"
    mode          = "0600"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
```

Collector pattern:

```text
/openbao/audit/audit.log
  -> Alloy file tail, read-only
  -> relabel log_stream="openbao.audit"
  -> Loki restricted tenant/folder
```

#### C. Production Kubernetes profile

```hcl
audit "file" "primary-file" {
  description = "Primary local audit device on dedicated PVC."

  options {
    file_path     = "/openbao/audit/audit.log"
    mode          = "0600"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}

audit "file" "secondary-stdout" {
  description = "Secondary audit stream for independent collection path."

  options {
    file_path     = "stdout"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
```

Optional HTTP audit device only after testing blocking behavior:

```hcl
audit "http" "audit-gateway" {
  description = "Optional internal audit gateway; production only after failure-mode tests."

  options {
    uri           = "https://openbao-audit-gateway.security.svc/ingest"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
```

Production profile requirements:

| Requirement                                              | Reason                                                       |
| -------------------------------------------------------- | ------------------------------------------------------------ |
| Dedicated audit PVC or host path with strict permissions | Prevent audit loss and separate operational/audit retention. |
| At least two audit paths                                 | OpenBao recommends multiple audit devices.                   |
| Separate archive pipeline                                | Loki is exploration, not immutable archive.                  |
| Disk/PVC alerts                                          | File audit failure or disk full can block requests.          |
| Failure-mode tests                                       | Network audit devices can block OpenBao.                     |
| Restricted Grafana and Loki access                       | HMAC does not remove all sensitive metadata.                 |

#### D. VM/systemd profile

```hcl
audit "file" "local-file" {
  description = "VM/systemd local audit file."

  options {
    file_path     = "/var/log/openbao/audit.log"
    mode          = "0600"
    format        = "json"
    hmac_accessor = "true"
    log_raw       = "false"
  }
}
```

Logrotate sketch:

```text
/var/log/openbao/audit.log {
  rotate 30
  daily
  missingok
  notifempty
  compress
  create 0600 openbao openbao
  postrotate
    /bin/kill -HUP $(cat /run/openbao/openbao.pid)
  endscript
}
```

## 10. Loki / LogQL design

### 10.1 Label strategy

Loki documentation emphasizes low-cardinality labels, warns that high-cardinality labels create large indexes and many tiny chunks, and recommends structured metadata for data that is too high-cardinality to index as labels. ([Grafana Labs][21])

Allowed bounded labels:

| Label                | Use                                                                                            |
| -------------------- | ---------------------------------------------------------------------------------------------- |
| `cluster`            | Required in multi-cluster/fleet setups.                                                        |
| `environment`        | `dev`, `stage`, `prod`, etc.                                                                   |
| `region`             | Bounded deployment region.                                                                     |
| `namespace`          | Kubernetes namespace; bounded in most platform setups.                                         |
| `app`                | Usually `openbao`.                                                                             |
| `component`          | `server`, `injector`, `collector`, etc.                                                        |
| `log_stream`         | `openbao.audit`, `openbao.operational`, etc.                                                   |
| `deployment_profile` | `demo`, `kubernetes-baseline`, `production-kubernetes`, `vm-systemd`.                          |
| `pod`                | Conditional. Useful for short-retention operational logs; prefer structured metadata at scale. |
| `container`          | Usually bounded.                                                                               |
| `instance`           | Conditional. Bounded VM identity or scrape instance; avoid unbounded churn.                    |

Forbidden labels:

| Forbidden label                                               | Reason                                                            |
| ------------------------------------------------------------- | ----------------------------------------------------------------- |
| `request_id`                                                  | Unique per request; use query-time filter or structured metadata. |
| `request_path`, `secret_path`, `mount_path`, `namespace_path` | Sensitive and high-cardinality.                                   |
| `client_token`, `token_accessor`                              | Sensitive security material.                                      |
| `entity_id`, `auth_accessor`                                  | Sensitive identity metadata; high-cardinality.                    |
| `client_ip`, `remote_address`                                 | High-cardinality and often personal data.                         |
| `policy`                                                      | Potentially sensitive and unbounded.                              |
| `user_name`, `display_name`                                   | Identity data; not labels.                                        |

Use labels for low-cardinality routing and coarse filtering. Use structured metadata for high-cardinality fields that are often used in drilldowns, such as `request_id` or pod UID, when the backend supports it. Use query-time JSON parsing for audit fields. Use Grafana derived fields for clickable request ID drilldowns without indexing request IDs as labels.

### 10.2 Example LogQL queries

Field names below are based on OpenBao docs, local OpenBao source, and v2.5.2/v2.5.4 declarative file-audit samples. Audit request IDs are nested at `request.id`, not top-level `request_id`. Loki's JSON parser can flatten nested properties with `_`, but explicit extraction keeps queries stable. ([openbao.org][2], [Grafana Labs][30], [GitHub][31])

| Use case | LogQL sketch |
| -------- | ------------ |
| Audit event volume | `sum by (cluster) (count_over_time({log_stream="openbao.audit",cluster=~"$cluster"}[5m]))` |
| Audit request volume | `sum by (cluster) (count_over_time({log_stream="openbao.audit",cluster=~"$cluster"} \| json \| type="request" [5m]))` |
| Audit response volume | `sum by (cluster) (count_over_time({log_stream="openbao.audit",cluster=~"$cluster"} \| json \| type="response" [5m]))` |
| Audit request/response balance | `sum(count_over_time({log_stream="openbao.audit"} \| json \| type="request" [10m])) - sum(count_over_time({log_stream="openbao.audit"} \| json \| type="response" [10m]))` |
| Failed audit responses | `sum by(cluster) (count_over_time({log_stream="openbao.audit"} \| json response_error="error" \| type="response" \| response_error!="" [5m]))` |
| High-risk `sys/*` paths | `{log_stream="openbao.audit"} \| json request_path="request.path" \| request_path=~"sys/(auth\|audit\|mounts\|policies\|raw\|plugins\|storage/raft\|rotate).*"` |
| Auth activity | `{log_stream="openbao.audit"} \| json request_path="request.path" \| request_path=~"auth/.*"` |
| System mutations | `{log_stream="openbao.audit"} \| json operation="request.operation", request_path="request.path" \| operation=~"create\|update\|delete" \| request_path=~"sys/.*"` |
| Operational error spikes | `sum by(cluster) (count_over_time({log_stream="openbao.operational"} \| json \| level=~"error\|warn" [5m]))` |
| Seal/unseal related logs | `{log_stream="openbao.operational"} \|~ "(?i)seal\|unseal"` |
| Raft/storage warnings | `{log_stream="openbao.operational"} \|~ "(?i)raft\|storage\|autopilot" \|~ "(?i)warn\|error\|failed"` |
| Missing audit stream | `absent_over_time({log_stream="openbao.audit",cluster="$cluster"}[10m])` |
| Request ID drilldown | `{log_stream=~"openbao.audit\|openbao.operational\|openbao.completed_requests"} \| json request_id="request.id" \| request_id="$request_id"` |

## 11. Alerting strategy

### 11.1 Severity model

| Severity | Meaning                                                                                                                                          |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| Critical | OpenBao may be unavailable, unsafe, unaudited, split-brained, or at immediate quorum/storage risk. Page immediately.                             |
| Warning  | Degradation, trend, misconfiguration, or risk that needs timely operator action but not immediate emergency unless combined with other symptoms. |
| Info     | Useful for dashboards, annotations, and maintenance awareness; not paging by default.                                                            |

### 11.2 Alert groups

Prometheus alert sketches use `${p}` for the configured metric prefix. Some queries require generated recording rules for production.

| Alert | Signal source | PromQL / LogQL sketch | Severity | Description | Suggested runbook URL | Known false positives | Required dashboard panel |
| ----- | ------------- | --------------------- | -------- | ----------- | --------------------- | --------------------- | ------------------------ |
| `OpenBaoUnreachable` | Metrics | `min_over_time(up{job=~"$job"}[5m]) == 0` | Critical | Prometheus cannot scrape OpenBao. | `docs/runbooks/openbao-metrics-scrape-failing.md` | Planned maintenance, scrape config rollout | Scrape health |
| `OpenBaoSealedUnexpectedly` | Metrics | `${p}_core_unsealed{cluster!=""} == 0` | Critical | Scraped OpenBao node is sealed. | `docs/runbooks/openbao-sealed-unexpectedly.md` | Planned seal/restart | Unsealed node count |
| `OpenBaoNoActiveNode` | Metrics | `sum(${p}_core_active) == 0` | Critical | No active node detected. | `docs/runbooks/no-active-openbao-leader.md` | Active-only scrape outage | Active node count |
| `OpenBaoMultipleActiveNodes` | Metrics | `sum(${p}_core_active) > 1` | Critical | More than one active node detected. | `docs/runbooks/multiple-active-nodes.md` | Stale series after restart; bad scrape dedupe | Active node count |
| `OpenBaoAuditRequestFailures` | Metrics | `sum(increase(${p}_audit_log_request_failure[5m])) > 0` | Critical | Audit request logging failed. | `docs/runbooks/audit-request-response-failures.md` | Counter reset handling if not using `increase` | Audit request failures |
| `OpenBaoAuditResponseFailures` | Metrics | `sum(increase(${p}_audit_log_response_failure[5m])) > 0` | Critical | Audit response logging failed. | `docs/runbooks/audit-request-response-failures.md` | Same as above | Audit response failures |
| `OpenBaoAuditStreamMissing` | Loki | `absent_over_time({log_stream="openbao.audit"}[10m])` | Critical | No audit logs received. | `docs/runbooks/audit-log-stream-missing.md` | Very idle demo clusters; collector rollout | Audit event volume |
| `OpenBaoAuditArchiveDegraded` | Archive/SIEM metric or log | `archive_delivery_success == 0` or backend-specific | Critical | Long-term audit archive degraded. | `docs/runbooks/audit-archive-degraded.md` | Archive maintenance window | Audit archive status |
| `OpenBaoRaftFailureToleranceLow` | Metrics | `min(${p}_autopilot_failure_tolerance) < 1` | Critical for HA | Raft has no healthy-node failure tolerance. | `docs/runbooks/failure-tolerance-below-threshold.md` | Single-node clusters | Failure tolerance |
| `OpenBaoAutopilotUnhealthy` | Metrics | `min(${p}_autopilot_healthy) == 0` | Critical | Autopilot reports unhealthy cluster. | `docs/runbooks/raft-peer-unhealthy.md` | During rolling restart | Autopilot healthy |
| `OpenBaoStorageBackendFailures` | Metrics/logs | LogQL: `{log_stream="openbao.operational"} \|~ "(?i)storage\|barrier\|raft" \|~ "(?i)error\|failed"` | Critical | Storage/barrier/Raft errors detected. | `docs/runbooks/storage-backend-failures.md` | Verbose but harmless retries; validate | Barrier/Raft/log panels |
| `OpenBaoAllPodsUnavailable` | Kubernetes metrics | `sum(kube_pod_container_status_ready{pod=~"openbao.*"}) == 0` | Critical | No OpenBao pod is ready. | `docs/runbooks/all-openbao-pods-unavailable.md` | Label mismatch | Pod readiness |
| `OpenBaoHighRequestLatency` | Metrics | `avg5(${p}_core_handle_request) > <baseline>` | Warning | Non-login latency above baseline. | `docs/runbooks/high-request-latency.md` | Load test, backup/restore | Request latency |
| `OpenBaoInflightRequestsRising` | Metrics | `deriv(sum(${p}_core_in_flight_requests)[15m:1m]) > 0` | Warning | In-flight requests rising. | `docs/runbooks/high-request-latency.md` | Traffic spike | In-flight requests |
| `OpenBaoLeadershipChurn` | Metrics | `sum(increase(${p}_raft_state_leader[30m])) > 1` | Warning | Repeated leadership changes. | `docs/runbooks/raft-peer-unhealthy.md` | Rolling restart | Leader transitions |
| `OpenBaoElevatedAuthFailures` | Audit logs | `{log_stream="openbao.audit"} \| json request_path="request.path", response_error="error" \| request_path=~"auth/.*" \| response_error!=""` | Warning | Auth failures above baseline. | `docs/runbooks/elevated-auth-failures.md` | Password spray tests, migration | Auth activity |
| `OpenBaoTokenGrowthAnomaly` | Metrics | `predict_linear(sum(${p}_token_count)[6h:10m], 3600) > <threshold>` | Warning | Token count projected above baseline. | `docs/runbooks/token-growth-anomaly.md` | New workload rollout | Token count |
| `OpenBaoLeaseGrowthAnomaly` | Metrics | `predict_linear(sum(${p}_expire_num_leases)[6h:10m], 3600) > <threshold>` | Warning | Lease count projected above baseline. | `docs/runbooks/lease-growth-anomaly.md` | Planned load increase | Lease count |
| `OpenBaoIrrevocableLeasesIncreasing` | Metrics | `increase(sum(${p}_expire_num_irrevocable_leases)[1h:10m]) > 0` | Warning | Irrevocable leases increased. | `docs/runbooks/irrevocable-leases.md` | Short-lived test engines | Irrevocable leases |
| `OpenBaoLeaseExpirationErrors` | Metrics | `sum(increase(${p}_expire_lease_expiration_error[15m])) > 0` | Warning | Lease expiration errors observed. | `docs/runbooks/lease-expiration-errors.md` | Backend outage already known | Lease expiration errors |
| `OpenBaoRuntimePressure` | Metrics | Goroutines/heap/sys bytes above baseline | Warning | Runtime pressure above baseline. | `docs/runbooks/runtime-pressure.md` | Load test | Runtime health |
| `OpenBaoCollectorDegraded` | Alloy/Prometheus/Loki | `up{job=~"alloy.*"} == 0` or Alloy self-metrics | Warning | Collector not healthy. | `docs/runbooks/loki-alloy-not-shipping.md` | Collector rollout | Collector status |
| `OpenBaoDebugTraceLoggingEnabled` | Logs/API probe | Log/API probe detects `debug` or `trace` logger | Warning | Debug/trace logging enabled. | `docs/runbooks/debug-logging-enabled.md` | Approved troubleshooting window | Operational logs |
| `OpenBaoCompletedRequestLoggingEnabled` | Logs | `count_over_time({log_stream="openbao.completed_requests"}[5m]) > 0` outside maintenance | Warning | Completed request logs enabled. | `docs/runbooks/completed-request-logging-enabled.md` | Approved troubleshooting window | Completed request stream |
| `OpenBaoAuditPVCPressure` | Kubernetes metrics | `kubelet_volume_stats_available_bytes / kubelet_volume_stats_capacity_bytes < 0.15` | Critical | Audit PVC running out of space. | `docs/runbooks/disk-pvc-pressure-openbao-audit.md` | Metrics lag | PVC/disk pressure |

## 12. Runbooks

Runbook docs should cite the relevant OpenBao pages for telemetry, audit behavior, file audit rotation, completed request logging, runtime loggers, and listener security. ([openbao.org][1])

| Runbook                                  | Symptoms                                                               | Impact                                                    | Likely causes                                                                                | Immediate checks                                                                           | Safe remediation steps                                                                                               | Escalation criteria                                            | Related dashboards / alerts / docs                                                 |
| ---------------------------------------- | ---------------------------------------------------------------------- | --------------------------------------------------------- | -------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| OpenBao sealed unexpectedly              | `vault_core_unsealed=0`, health failing, clients receive sealed errors | OpenBao cannot serve normal requests                      | Manual seal, auto-unseal/KMS/HSM issue, restart with seal backend unavailable, storage issue | `bao status`, `/sys/seal-status`, operational logs, KMS/HSM health, pod restarts           | Restore seal backend, use approved unseal procedure, verify audit stream after recovery                              | Unseal keys unavailable, suspected compromise, repeated reseal | Overview; `OpenBaoSealedUnexpectedly`; audit docs note seal paths are not audited. |
| No active OpenBao leader                 | Active count `0`, clients redirect/fail, Raft churn                    | Cluster unavailable or degraded                           | Raft quorum loss, storage backend failure, network partition, all nodes standby/sealed       | `/sys/leader`, Raft peer list, Autopilot metrics, pod readiness, storage logs              | Restore quorum/network/storage; restart failed nodes one at a time; avoid destructive Raft operations without backup | Quorum cannot be restored, split-brain suspected               | HA/Raft; `OpenBaoNoActiveNode`; Raft metrics docs.                                 |
| Multiple active nodes detected           | Active count `>1`                                                      | Possible split-brain or scrape artifact                   | Network partition, stale metrics, duplicate cluster label, scrape dedupe issue               | Query by `instance`, compare `/sys/leader`, check network partition, inspect scrape labels | If true split-brain, isolate affected nodes and follow incident process; if scrape artifact, fix labels/staleness    | More than one node accepts writes                              | Overview/HA; `OpenBaoMultipleActiveNodes`.                                         |
| OpenBao metrics scrape failing           | `up=0`, missing panels                                                 | Loss of monitoring/alerts                                 | Token expired, TLS/CA error, path/params wrong, endpoint active-only, NetworkPolicy          | Prometheus target errors, curl `/v1/sys/metrics?format=prometheus`, token capability       | Renew metrics token, fix CA/path/params, restore route, validate standby behavior                                    | Monitoring blind > alert threshold                             | Overview; `OpenBaoUnreachable`; telemetry docs.                                    |
| Audit request/response failures          | Audit failure counters increase                                        | Requests may fail; audit evidence incomplete              | Disk full, file permission, bad HTTP/syslog/socket sink, blocked device                      | Audit metrics, OpenBao logs, device-specific metrics, PVC/disk, collector status           | Restore at least one local file device; fix permissions/disk; avoid disabling all audit without security approval    | Any production audit gap or raw logging                        | Audit Overview; audit failure alerts; audit docs.                                  |
| Audit log stream missing                 | Loki has no `openbao.audit` stream                                     | Security blind spot; archive may still exist or be broken | Collector down, wrong labels, file path changed, audit device disabled, no activity          | Check audit device list, file mtime/size, Alloy status, Loki labels, archive delivery      | Restore collector labels/path; verify audit file receives new entry; confirm archive                                 | Missing production audit stream > 10m                          | Audit Overview; `OpenBaoAuditStreamMissing`.                                       |
| Audit device blocked or degraded         | Requests hang, audit latency high, failures                            | OpenBao request path blocked or slow                      | Blocking network device, slow HTTP audit endpoint, syslog/socket unavailable, disk stall     | Audit latency, goroutines, operational logs, sink health, packet loss                      | Restore sink, remove blocking network path only through approved config change, ensure local file path active        | Cluster request hang or audit loss                             | Audit Overview; audit blocking docs.                                               |
| Loki/Alloy collector not shipping logs   | No new logs/events, collector alerts                                   | Loss of log exploration/alerts                            | Alloy crash, RBAC, bad Loki credentials, network, file permissions                           | Alloy pod logs, Alloy self-metrics, Loki write errors, RBAC, file mounts                   | Roll back Alloy config, restore credentials, fix RBAC/mounts, replay from positions if possible                      | Audit archive affected                                         | Operational Logs; `OpenBaoCollectorDegraded`; Alloy docs.                          |
| Raft peer unhealthy                      | Autopilot unhealthy, follower lag, peer count mismatch                 | HA risk, quorum loss                                      | Node down, network, disk latency, version skew, storage corruption                           | Autopilot metrics, Raft peer list, pod/node health, PVC latency                            | Restore node/network/disk; controlled restart; do not remove peer without backup/process                             | Failure tolerance zero or quorum lost                          | HA/Raft; `OpenBaoAutopilotUnhealthy`.                                              |
| Failure tolerance below threshold        | `autopilot_failure_tolerance < 1`                                      | Next failure may lose quorum                              | Node unhealthy, planned maintenance, odd/even peer issue                                     | Healthy peer count, expected replicas, pod/node/PVC state                                  | Pause maintenance, restore failed node, scale only following OpenBao guidance                                        | Production HA tolerance zero outside maintenance               | HA/Raft; `OpenBaoRaftFailureToleranceLow`.                                         |
| High request latency                     | Core latency and in-flight requests rising                             | Client timeouts, auth delays                              | Storage latency, audit latency, CPU/mem, token checks, Raft lag                              | Core latency, audit latency, barrier latency, CPU/mem, logs                                | Reduce load, fix audit/storage bottleneck, scale clients back, investigate auth backend                              | SLO burn or request failures                                   | Overview/SLO; `OpenBaoHighRequestLatency`.                                         |
| Token count growing unexpectedly         | Token count trend above baseline                                       | Token store pressure, security risk                       | Leaking app tokens, missing revocation, new auth workload                                    | Token count, creation rate by auth, audit auth activity                                    | Identify auth method/workload; rotate/revoke through approved process                                                | Root/admin tokens or policy abuse suspected                    | Token/Lease; `OpenBaoTokenGrowthAnomaly`.                                          |
| Lease expiration errors                  | Expiration error counter increases                                     | Dynamic secrets may not revoke                            | Backend unreachable, plugin/engine error, lease backend issue                                | Expiration errors, engine logs, storage/barrier latency                                    | Restore backend, manually revoke only with owner approval                                                            | Credential leak risk                                           | Token/Lease; `OpenBaoLeaseExpirationErrors`.                                       |
| Irrevocable leases increasing            | Irrevocable gauge rises                                                | Secrets cannot be automatically revoked                   | Revocation backend/plugin failure, deleted role/mount, backend unavailable                   | Irrevocable count, lease engine, audit mutations                                           | Restore engine/backend; coordinate manual cleanup                                                                    | External credentials remain valid                              | Token/Lease; `OpenBaoIrrevocableLeasesIncreasing`.                                 |
| Debug logging enabled                    | Logger level debug/trace or high-volume logs                           | Sensitive metadata exposure, cost                         | Runtime logger API, config change, troubleshooting left on                                   | `/sys/loggers`, config, logs, change records                                               | Revert logger via API or config + SIGHUP; review retention/access                                                    | Debug logs persisted with sensitive data                       | Operational Logs; `OpenBaoDebugTraceLoggingEnabled`.                               |
| Completed request logging enabled        | `openbao.completed_requests` stream has events                         | Sensitive request metadata exposure                       | `log_requests_level` set during troubleshooting                                              | Config, SIGHUP history, stream volume                                                      | Set `log_requests_level="off"`, SIGHUP, enforce retention deletion policy                                            | Enabled outside approved window                                | Operational Logs; completed request docs.                                          |
| Disk/PVC pressure for OpenBao/audit logs | PVC below threshold, write errors                                      | Audit blocking, storage instability                       | Audit growth, no rotation, archive lag, disk leak                                            | PVC metrics, file size, logrotate, archive success                                         | Expand volume, rotate/reopen, restore archive, reduce noisy logging                                                  | Audit file cannot be written                                   | Platform; `OpenBaoAuditPVCPressure`; file audit docs.                              |

## 13. Deployment profiles

### A. Local demo

Purpose: fast development and screenshots, not security validation.

Components:

```text
Docker Compose or kind
  ├─ OpenBao dev or single-node server
  ├─ Prometheus
  ├─ Loki
  ├─ Alloy
  └─ Grafana with provisioned dashboards
```

Settings:

| Area      | Demo decision                                                                                    |
| --------- | ------------------------------------------------------------------------------------------------ |
| Metrics   | `prometheus_retention_time="30s"`, `disable_hostname=true`, `metrics_prefix="vault"` by default. |
| Scrape    | Active/single-node scrape.                                                                       |
| Audit     | File audit to stdout with `log_raw=false`.                                                       |
| Logs      | Operational and audit streams relabeled separately.                                              |
| Retention | Short.                                                                                           |
| Caveat    | Not compliance, not HA, not all-node behavior.                                                   |

### B. Kubernetes baseline

Components:

```text
OpenBao on Kubernetes
  -> active ServiceMonitor
  -> Alloy DaemonSet for pod logs/events
  -> optional audit PVC or stdout audit
  -> Loki
  -> Grafana dashboards
  -> PrometheusRule
```

Decisions:

| Area     | Baseline                                                                                |
| -------- | --------------------------------------------------------------------------------------- |
| Metrics  | Authenticated active-only scrape preferred.                                             |
| Audit    | Mounted file preferred; stdout allowed for simpler baseline with restricted collection. |
| Labels   | Low-cardinality labels only.                                                            |
| Platform | kube-state-metrics, kubelet/cAdvisor, Kubernetes events.                                |
| Runbooks | Basic critical runbooks included.                                                       |

### C. Production Kubernetes

Required controls:

| Area       | Production requirement                                                                                   |
| ---------- | -------------------------------------------------------------------------------------------------------- |
| OpenBao    | HA OpenBao with Raft or supported HA backend.                                                            |
| Metrics    | Secure active scrape plus optional all-node metrics-only listener.                                       |
| Access     | NetworkPolicy, RBAC, TLS, restricted metrics token.                                                      |
| Audit      | At least two audit paths; `log_raw=false`; restricted audit folder/datasource.                           |
| Archive    | SIEM/object store/WORM archive path separate from Loki.                                                  |
| Storage    | Data PVC and audit PVC monitored.                                                                        |
| Collectors | Alloy DaemonSet/sidecar with least privilege.                                                            |
| Alerts     | PrometheusRule and/or Grafana alerting enabled.                                                          |
| Tests      | Failure-mode tests for disk full, collector down, sink down, token expiry, standby scrape, label linter. |

### D. VM/systemd

Components:

```text
openbao.service
  ├─ journald or log_file operational logs
  ├─ file audit device
  ├─ Alloy system service
  ├─ node_exporter
  ├─ Prometheus scrape
  └─ Loki or alternative backend
```

Requirements:

| Area             | VM/systemd profile                                                              |
| ---------------- | ------------------------------------------------------------------------------- |
| Operational logs | journald preferred; JSON format when practical.                                 |
| Audit            | `/var/log/openbao/audit.log`, mode `0600`, logrotate with SIGHUP.               |
| Metrics          | Localhost/private metrics listener with token or private metrics-only listener. |
| Host             | node_exporter disk, CPU, memory, file-system metrics.                           |
| Tests            | Rotation reopen, file permission denial, disk pressure, collector restart.      |

### E. Bring-your-own observability

This profile ships contracts and generated artifacts only:

| Existing backend                        | Project provides                                                         |
| --------------------------------------- | ------------------------------------------------------------------------ |
| Prometheus/Mimir/Thanos/VictoriaMetrics | Scrape contract, recording rules, alert rules, metric prefix config.     |
| Loki/OpenSearch/Splunk/Elastic/SIEM     | Stream contract, label policy, parsing examples, forbidden-label checks. |
| Grafana                                 | Dashboard JSON, provisioning examples, folders, data source assumptions. |
| Alertmanager/Grafana alerting           | PrometheusRule and Grafana alert definitions where applicable.           |

## 14. Repository structure

```text
openbao-observability/
├── README.md
├── docs/
│   ├── architecture.md
│   ├── contracts/
│   │   ├── openbao-overview.md
│   │   ├── openbao-logging.md
│   │   ├── openbao-audit.md
│   │   └── openbao-alerts.md
│   ├── dashboards/
│   ├── logging/
│   ├── metrics/
│   ├── runbooks/
│   ├── deployment-profiles/
│   └── security-model.md
├── contracts/
│   ├── dashboards/
│   │   ├── openbao-overview.yaml
│   │   ├── openbao-audit-overview.yaml
│   │   └── openbao-ha-raft.yaml
│   ├── alerts/
│   │   ├── prometheus.yaml
│   │   └── loki.yaml
│   └── streams/
│       ├── logs.yaml
│       └── audit.yaml
├── dashboards/
│   └── grafana/
│       ├── generated/
│       └── provisioning/
├── alerts/
│   ├── prometheus/
│   ├── loki/
│   └── grafana/
├── alloy/
│   ├── kubernetes/
│   ├── systemd/
│   └── docker-compose/
├── examples/
│   ├── docker-compose/
│   ├── kind/
│   ├── kubernetes/
│   └── production/
├── helm/
├── jsonnet/
│   ├── lib/
│   └── dashboards/
├── generated/
│   ├── dashboards/
│   ├── prometheusrules/
│   └── docs/
├── cmd/
│   └── openbao-observability/
├── internal/
│   ├── fixtures/
│   ├── dashboards/
│   └── rules/
├── fixtures/
│   ├── metrics/
│   │   ├── openbao-2.5-vault-prefix.prom
│   │   ├── openbao-2.5-openbao-prefix.prom
│   │   └── openbao-2.5-raft.prom
│   └── logs/
│       ├── audit/
│       ├── operational/
│       └── completed-requests/
├── tests/
│   ├── promtool/
│   ├── dashboard-lint/
│   ├── logql/
│   ├── contracts/
│   ├── fixtures/
│   └── docs/
├── go.mod
└── Makefile
```

### Dashboard maintenance recommendation

Grafana dashboards are JSON objects, and Grafana provisioning can load version-controlled dashboards and data sources from files. Grafana also documents alert provisioning through files. ([Grafana Labs][22])

| Approach                   | Recommendation                                     | Pros                                                                                               | Cons                                                                                                 |
| -------------------------- | -------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Plain JSON                 | Release artifact only                              | Easy import, universal                                                                             | Poor reviewability, hard to keep contracts/tests synchronized.                                       |
| Jsonnet + Grafonnet        | Recommended initial implementation path            | Mature dashboard-as-code workflow; Grafonnet is generated from Grafana OpenAPI/Foundation SDK docs | Jsonnet learning curve; generated output still needs linting. ([GitHub][23])                         |
| Grafana Foundation SDK     | Strong candidate for later versions                | Strongly typed code; official CI/CD dashboard automation path                                      | More language/tooling choices; ecosystem still evolving. ([Grafana Labs][24])                        |
| Grizzly                    | Optional local dev/apply tool, not core dependency | Useful review/publish workflow for generated JSON/YAML                                             | Grafana’s Grizzly repo notes deprecation/supersession by grafanactl. ([GitHub][25])                  |
| Terraform provider Grafana | Deployment option                                  | Good for managed Grafana instances and RBAC/folders                                                | Terraform state is not ideal for reusable OSS dashboard source.                                      |
| Tanka                      | Optional full-stack demo deployment                | Good for Jsonnet-based Kubernetes environments                                                     | Overkill for users who only want contracts/dashboards.                                               |
| Helm                       | Packaging, not dashboard source                    | Good for Kubernetes examples                                                                       | Templated dashboards can drift; current OpenBao Helm issue shows label mismatch risk. ([GitHub][26]) |

Recommended source model:

```text
contracts/*.yaml
  -> generator
     ├─ Grafana dashboard JSON
     ├─ PrometheusRule YAML
     ├─ Loki/Grafana alert YAML
     ├─ docs tables
     └─ fixture tests
```

## 15. CI and validation strategy

| Validation                | Tooling                             | Required checks                                                                                     |
| ------------------------- | ----------------------------------- | --------------------------------------------------------------------------------------------------- |
| Prometheus rule syntax    | `promtool check rules`              | All generated rules valid.                                                                          |
| PromQL fixture evaluation | `promtool test rules`               | Critical alerts fire/not fire against known fixtures.                                               |
| Dashboard JSON            | Grafana JSON schema / custom linter | JSON valid; panels have contract IDs; no orphan queries.                                            |
| Contract schema           | JSON Schema / CUE / OpenAPI         | Required fields: purpose, signal, docs metric, PromQL/LogQL, caveats, security notes.               |
| LogQL syntax              | `logcli` or ephemeral Loki          | Queries parse and return expected fixture results.                                                  |
| Audit fixture tests       | Captured audit JSON                 | Request/response parsing, missing stream alert, risky path queries.                                 |
| Metric fixture tests      | Captured Prometheus text            | Metric exists, labels expected, unit conventions verified.                                          |
| Prefix tests              | `vault` and `openbao` fixtures      | Generated dashboards/rules work for both prefixes.                                                  |
| Forbidden labels          | Static linter                       | Reject Loki label matchers/groupings using forbidden fields.                                        |
| Cardinality checks        | Static + fixture                    | Reject dashboards grouping by policy/path/entity/request/client IP unless explicitly restricted.    |
| Docs links                | Link checker                        | All cited docs and internal runbook links valid.                                                    |
| Security policy checks    | Regex/CUE                           | Reject `log_raw=true`, default `debug`/`trace`, completed request logging enabled in baseline/prod. |
| Compatibility matrix      | Generated markdown                  | OpenBao version, prefix, feature flags, observed metrics, missing metrics.                          |

Sample fixture generation:

```bash
# Metrics fixture
curl \
  --header "X-Vault-Token: ${OPENBAO_METRICS_TOKEN}" \
  "https://openbao.example:8200/v1/sys/metrics?format=prometheus" \
  > fixtures/metrics/openbao-${OPENBAO_VERSION}-${PREFIX}.prom

# Exercise basic signals
# For OpenBao v2.5+, start OpenBao with a declarative audit stanza.
# API audit creation requires unsafe_allow_api_audit_creation=true and
# should only be used by an isolated negative/unsafe test fixture.
bao secrets enable -path=demo kv-v2
bao kv put demo/example value=test
bao auth enable userpass
bao token create -ttl=10m

# Capture audit fixture
cp /tmp/openbao-audit.log fixtures/logs/audit/openbao-basic.jsonl
```

For HA/Raft fixtures, use a kind profile with three OpenBao pods and integrated storage. Dev mode is acceptable for basic metrics and audit examples, but not for HA/Raft validation.

## 16. Security model

| Area                            | Threat                                                                                         | Control                                                                                                                                                                                                          |
| ------------------------------- | ---------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Metrics endpoint exposure       | Metrics reveal topology, usage, auth methods, mounts, pressure, and sometimes sensitive labels | Authenticated active scrape by default; private metrics-only listener only with network isolation; scoped metrics token; TLS.                                                                                    |
| Metrics token                   | Token theft allows metrics reads                                                               | Store token in Kubernetes Secret or external secret manager; rotate; minimal `sys/metrics` policy; no broad OpenBao policy.                                                                                      |
| Standby metrics                 | OpenBao standby metrics require unauthenticated metrics access                                 | Use only in all-node profile with metrics-only listener, NetworkPolicy/firewall, private address, optional mTLS proxy. ([openbao.org][1])                                                                        |
| Sensitive metric labels         | Policy/mount/auth labels may leak design or tenant data                                        | Restrict detailed dashboards; do not group by sensitive labels in overview; use allowlists.                                                                                                                      |
| Audit logs                      | HMAC does not make audit logs safe for broad access                                            | Dedicated stream, restricted Loki tenant/folder, SIEM/archive, short exploration retention, least-privilege access.                                                                                              |
| `log_raw=true`                  | Plaintext secrets in audit logs                                                                | CI rejection; never in normal profiles; break-glass only in isolated test with cleanup.                                                                                                                          |
| Loki access                     | Developers can infer paths/users/actions                                                       | Separate datasource/folder permissions; no broad Explore access to `openbao.audit`; audit queries reviewed.                                                                                                      |
| Grafana access                  | Dashboard viewers can query sensitive labels/logs                                              | Folder permissions, datasource permissions, query limits, separate security dashboards.                                                                                                                          |
| Retention/deletion              | Sensitive metadata retained too long or deleted too soon                                       | Separate exploration retention and compliance retention; legal/security policy outside repo.                                                                                                                     |
| Tamper resistance               | Compromised node/collector/backend can alter logs                                              | Multiple audit devices, archive/SIEM path, object-lock/WORM where required, integrity checks.                                                                                                                    |
| Collector privileges            | Alloy DaemonSet can read node logs, pod logs, audit files                                      | Separate service account, minimal RBAC, read-only mounts, no write to audit volume, restricted namespace scope where possible.                                                                                   |
| Kubernetes RBAC                 | Collector can watch events/secrets accidentally                                                | Grant only pods/logs/events/nodes as needed; never secrets unless required by deployment. Alloy Kubernetes events need watch permissions at cluster scope unless namespaces are restricted. ([Grafana Labs][13]) |
| Remote write / Loki credentials | Credential theft exfiltrates logs/metrics                                                      | Store in secrets, rotate, scope by tenant, use TLS, avoid embedding in generated files.                                                                                                                          |
| Compromised Grafana user        | Query abuse across audit logs                                                                  | Limit Explore, restrict audit datasource, add audit trail for Grafana itself where available, separate admin roles.                                                                                              |
| Compromised collector           | Exfiltrates audit logs                                                                         | Network egress allowlist, dedicated credentials, runtime hardening, image pinning, no shell/debug in production.                                                                                                 |
| Completed request logs          | Request metadata exposure                                                                      | Disabled by default; explicit approved window; short retention; alert when enabled.                                                                                                                              |

## 17. MVP plan

### v0.1

| Deliverable                          | Scope                                                                                    |
| ------------------------------------ | ---------------------------------------------------------------------------------------- |
| `docs/contracts/openbao-overview.md` | Overview contract with core/audit/runtime/token/lease/Raft/platform panels.              |
| `docs/contracts/openbao-logging.md`  | Stream definitions and Loki label policy.                                                |
| `docs/contracts/openbao-audit.md`    | Audit profile design and baseline configs.                                               |
| Telemetry config examples            | Secure active scrape and dedicated metrics listener examples.                            |
| Prometheus scrape / ServiceMonitor   | Active-only baseline.                                                                    |
| Grafana OpenBao Overview             | Generated from initial contract.                                                         |
| Grafana OpenBao Audit Overview       | Audit failures, latency, volume, missing stream.                                         |
| Alloy Kubernetes log config          | Pod logs, audit file/stdout stream, Kubernetes events.                                   |
| Basic PrometheusRule alerts          | Unreachable, sealed, no/multiple active, audit failures, Raft failure tolerance.         |
| Basic Loki alerts                    | Missing audit stream, operational error spike, completed request logging stream present. |
| Demo deployment                      | Docker Compose or kind.                                                                  |
| Initial runbooks                     | Critical runbooks from section 12.                                                       |

### v0.2

| Deliverable                   | Scope                                                          |
| ----------------------------- | -------------------------------------------------------------- |
| HA/Raft dashboard             | Autopilot, peer, election, follower, snapshot, storage panels. |
| Operational Logs dashboard    | Error/warn, seal/unseal, Raft/storage log exploration.         |
| Audit Investigation dashboard | Request ID drilldown, risky sys paths, auth activity.          |
| Improved LogQL examples       | Validated against audit fixtures.                              |
| VM/systemd profile            | journald/file/logrotate/Alloy/node exporter.                   |
| Dashboard generation pipeline | Contract to Grafana JSON/docs.                                 |
| Fixture tests                 | Metrics and audit samples.                                     |

### v0.3

| Deliverable                    | Scope                                                                      |
| ------------------------------ | -------------------------------------------------------------------------- |
| Auth/Identity dashboard        | Login latency, entity/alias metrics, auth activity.                        |
| Token/Lease dashboard          | Token count, creation, TTL, leases, irrevocable leases, expiration errors. |
| Database Secrets dashboard     | Database operation latency, failures, lease creation, and audit streams.    |
| PKI/Transit dashboards         | Feature-specific contracts with restricted labels.                         |
| Production Kubernetes profile  | NetworkPolicy, RBAC, metrics listener, audit PVC/archive.                  |
| OpenBao Operator contract      | Resource, label, scrape, log, dashboard, and alert boundary contract.      |
| OpenBao Operator examples      | Active scrape, all-node scrape, audit, and artifact adoption examples.     |
| Audit archive reference design | SIEM/object store/WORM path examples in `docs/audit/audit-archive-reference-design.md`. |
| Stronger CI                    | Forbidden-label linter, prefix tests, dashboard schema.                    |

### v0.4

| Deliverable                   | Scope                                                           |
| ----------------------------- | --------------------------------------------------------------- |
| SLO dashboards                | Availability, latency, burn alerts, synthetic probes.           |
| Multi-cluster/fleet variables | Cluster/environment/region rollups.                             |
| Compatibility matrix          | OpenBao version, metrics, labels, caveats.                      |
| OpenBao Operator validation   | Live operator-managed staging validation and read-replica fixtures. |
| Advanced security profile     | mTLS metrics proxy, object-lock archive, stricter Grafana/RBAC. |

## 18. Resolved findings and validation backlog

### 18.1 Resolved or partially answered during this review

| Topic | Answer | Remaining validation |
| ----- | ------ | -------------------- |
| Latest OpenBao release | GitHub reports `v2.5.4` as the latest OpenBao release, published 2026-05-20; the container image reports `OpenBao v2.5.4`, built 2026-05-20. ([GitHub][33]) | Add automated release-version checks only if the project publishes compatibility badges. |
| Current Helm chart | GitHub reports Helm chart `openbao-0.28.3`, published 2026-05-21, with an OpenBao `v2.5.4` bump. Current chart source has `version: 0.28.3` and `appVersion: v2.5.4`. ([GitHub][28], [GitHub][34]) | Render chart versions in CI and pin tested versions in the compatibility matrix. |
| `metrics_prefix="openbao"` | Observed OpenBao v2.5.2 and v2.5.4 emitted expected `openbao_*` names for common core, audit, lease, and runtime metrics. | Full feature matrix still needed across Raft, auth, KV, PKI, transit, and both prefixes. |
| Audit request ID location | Local OpenBao source plus v2.5.2 and v2.5.4 audit samples show request IDs nested at `request.id`, not top-level `request_id`. ([GitHub][31]) | Fixture-test derived fields and LogQL extraction across common operations. |
| Audit JSON field shape | Audit entries have top-level `type`, `auth`, `request`, `response`, and `error` fields, with request details under `request.path`, `request.operation`, `request.mount_point`, `request.mount_type`, and `request.namespace.id`. ([openbao.org][2], [GitHub][31]) | Capture request/response fixtures for auth, sys, KV, token, and error cases. |
| Helm ServiceMonitor profile | The chart ServiceMonitor selects `openbao-active: "true"` in HA mode and scrapes `/v1/sys/metrics` with `format=prometheus`. ([GitHub][29]) | Add chart-render tests for active-only and any future all-node profile. |
| Per-device audit metrics | Observed v2.5.2 and v2.5.4 emitted aggregate audit failures and per-device audit latency metrics such as `vault_audit_local_file__log_request` / `openbao_audit_local_file__log_request`; per-device failure counters were not observed. | Verify device-name sanitization across more device names and profiles. |
| API audit creation | Observed v2.5.2 and v2.5.4 reject `bao audit enable ...` by default unless `unsafe_allow_api_audit_creation=true`; declarative audit config is the safe baseline. ([openbao.org][15], [openbao.org][20]) | Add negative test proving API audit creation is disabled by default. |
| Raw metric label sets | Observed v2.5.2 and v2.5.4 label sets are mixed: some metrics include `cluster`, some are unlabeled, and `core_unsealed` can emit an extra `cluster=""` series. | Recording-rule generation should normalize labels from scrape metadata, then fixture-test raw labels by version/profile. |

### 18.2 Remaining validation backlog

| Question | Why it matters | Validation method |
| -------- | -------------- | ----------------- |
| Actual Prometheus names for every docs metric by OpenBao version | Dots, uppercase, hyphens, and device names may sanitize differently | Capture `/v1/sys/metrics?format=prometheus` fixtures. |
| Which metrics exist only when features/storage backends are enabled | Avoid broken panels | Feature matrix: dev, Raft, audit enabled, KV, auth, PKI, transit. |
| Standby metrics behavior under different listener configs | All-node profile depends on this | Test authenticated scrape, unauth scrape, metrics-only listener, PodMonitor. |
| Which route/mount/policy metrics create unsafe cardinality | Dashboard safety | Cardinality analysis against realistic clusters. |
| How audit device blocking behaves under sink failures | Production safety | Controlled tests: full disk, permission denied, HTTP sink down/slow, TCP socket down. |
| How Loki structured metadata should be emitted by Alloy for audit request IDs | Better drilldown without labels | Test Alloy pipeline with Loki schema supporting structured metadata. |
| Which dashboard generation approach is most maintainable | Long-term OSS maintenance | Prototype Jsonnet/Grafonnet and Foundation SDK from same contract. |
| Whether OpenBao docs examples using `/dev/stdout` and file-device `stdout` behave identically in declarative audit config | Avoid broken container audit profile | Test both in supported OpenBao versions. |
| Which OpenBao logs reliably indicate debug/trace and completed request logging changes | Alert reliability | Runtime logger and config reload fixtures. |
| Whether `vault.secret.kv.count` availability is version/feature dependent | Prior art issue reports missing secret metrics | Reproduce with KV v1/v2 and version matrix. ([GitHub][27]) |
| Grafana/Loki permissions model for restricted audit folders/datasources | Security model implementation | Provide OSS and Enterprise/Cloud variants where possible. |

[1]: https://openbao.org/docs/configuration/telemetry/ "telemetry stanza | OpenBao"
[2]: https://openbao.org/docs/audit/ "Audit devices | OpenBao"
[3]: https://grafana.com/docs/loki/latest/send-data/promtail/ "Promtail agent | Grafana Loki documentation"
[4]: https://grafana.com/grafana/dashboards/23725-openbao/ "OpenBao | Grafana Labs"
[5]: https://openbao.org/docs/configuration/log-requests-level/ "Log completed requests"
[6]: https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/pod-metrics.md "kube-state-metrics/docs/metrics/workload/pod-metrics.md at main · kubernetes/kube-state-metrics · GitHub"
[7]: https://openbao.org/docs/platform/k8s/helm/ "Helm chart | OpenBao"
[8]: https://openbao.org/docs/audit/file/ "File audit device"
[9]: https://openbao.org/docs/configuration/listener/tcp/ "tcp listener"
[10]: https://prometheus-operator.dev/docs/api-reference/api/ "API reference - Prometheus Operator"
[11]: https://openbao.org/api-docs/system/metrics/ "sys/metrics"
[12]: https://openbao.org/docs/internals/telemetry/metrics/all/ "All OpenBao telemetry metrics | OpenBao"
[13]: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.kubernetes_events/ "loki.source.kubernetes_events | Grafana Alloy documentation"
[14]: https://grafana.com/docs/alloy/latest/collect/logs-in-kubernetes/ "Collect Kubernetes logs and forward them to Loki | Grafana Alloy documentation"
[15]: https://openbao.org/docs/configuration/ "OpenBao configuration"
[16]: https://openbao.org/api-docs/system/loggers/ "sys/loggers"
[17]: https://openbao.org/docs/audit/http/ "HTTP audit device"
[18]: https://openbao.org/docs/audit/syslog/ "Syslog audit device"
[19]: https://openbao.org/docs/audit/socket/ "Socket audit device"
[20]: https://openbao.org/docs/configuration/audit/ "Declarative Audit Devices"
[21]: https://grafana.com/docs/loki/latest/get-started/labels/ "Understand labels | Grafana Loki documentation"
[22]: https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/ "Dashboard JSON model - Grafana documentation"
[23]: https://github.com/grafana/grafonnet "grafana/grafonnet: Jsonnet library for generating ..."
[24]: https://grafana.com/docs/grafana/latest/as-code/observability-as-code/foundation-sdk/dashboard-automation/ "Automate dashboard provisioning with CI/CD"
[25]: https://github.com/grafana/grizzly "grafana/grizzly: A utility for managing Jsonnet dashboards ..."
[26]: https://github.com/openbao/openbao-helm/issues/111 "Generated grafana dashboard does not reflect labels in metrics · Issue #111 · openbao/openbao-helm · GitHub"
[27]: https://github.com/openbao/openbao/issues/2611 "Missing Secrets Metrics · Issue #2611 · openbao/openbao · GitHub"
[28]: https://github.com/openbao/openbao-helm/blob/main/charts/openbao/Chart.yaml "openbao-helm Chart.yaml"
[29]: https://github.com/openbao/openbao-helm/blob/main/charts/openbao/templates/prometheus-servicemonitor.yaml "openbao-helm ServiceMonitor template"
[30]: https://grafana.com/docs/loki/latest/query/log_queries/ "Log queries | Grafana Loki documentation"
[31]: https://github.com/openbao/openbao/blob/main/audit/format.go "OpenBao audit format source"
[32]: https://github.com/openbao/openbao-helm/blob/main/charts/openbao/values.yaml "openbao-helm values.yaml"
[33]: https://github.com/openbao/openbao/releases/tag/v2.5.4 "OpenBao v2.5.4 release"
[34]: https://github.com/openbao/openbao-helm/releases/tag/openbao-0.28.3 "openbao-helm 0.28.3 release"
[35]: https://openbao.org/docs/commands/namespace/ "namespace command"
[36]: https://openbao.org/docs/internals/limits/ "OpenBao limits"
[37]: https://openbao.org/docs/configuration/storage/raft/ "Raft storage configuration"
[38]: https://openbao.org/docs/commands/operator/raft/ "Raft operator command"
