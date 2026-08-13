# Metrics, logs, and audit logs

Use this explainer to decide whether a question belongs in OpenBao metrics,
operational logs, audit logs, platform signals, or dashboards. It is for
operators who need to avoid mixing troubleshooting signals with security
records.

## Why this matters

OpenBao emits different signals for different reasons. If you use the wrong
signal, you either miss the problem or expose data that did not need to leave a
restricted path.

The safest design starts with a specific question. Choose the narrowest signal
that answers that question, then decide which dashboard, alert, or runbook
needs it.

## Mental model

| Signal | Best for | Not good for | Example question |
| ------ | -------- | ------------ | ---------------- |
| Metrics | Health, rate, latency, saturation, and inventory trends. | Full forensic reconstruction. | Is request latency rising on the active node? |
| Operational logs | Server behavior and troubleshooting. | Compliance evidence. | Did Raft report storage or leadership problems? |
| Audit logs | API request and response security records. | General process debugging. | Which audited request path did this token access? |
| Platform signals | Runtime and orchestration context. | OpenBao internal state. | Did the pod restart or lose its audit volume? |
| Dashboards | Fast interpretation across signals. | Authoritative evidence by themselves. | Which dashboard narrows the incident scope? |

## Metrics

Metrics are numeric measurements. OpenBao exposes counters, gauges, and
summaries through telemetry. The Prometheus profile scrapes
`/v1/sys/metrics` with `format=prometheus`.

Use metrics when you need:

- Current health and state, such as seal, active node, or Raft health.
- Rates, such as token creation or audit failures over a time window.
- Latency summaries, such as request handling, login handling, or token checks.
- Inventory trends, such as token, lease, entity, or mount counts.
- Runtime pressure, such as goroutines, memory, and process behavior.

Do not expect every metric to update continuously. OpenBao high-cardinality
usage gauges update on `usage_gauge_period`, which defaults to 10 minutes.

Metrics are also not security records. A counter can prove that audit failures
increased. It cannot reconstruct the original request.

## Operational logs

Operational logs describe what the OpenBao server process does. Use them for
startup, shutdown, listener, TLS, storage, Raft, plugin, and process-level
troubleshooting.

Completed request logging is a separate operational logging feature. It is
disabled by default. If you enable it, OpenBao logs completed requests at the
configured `log_requests_level` only when the main `log_level` includes that
level.

Use completed request logs for temporary troubleshooting. Do not use them as an
audit-log replacement.

## Audit logs

Audit logs are security records for audited OpenBao API request and response
entries. They help you investigate API activity, correlate request and response
records, and confirm that expected audited actions reach an audit device.

Audit logs do not cover every system path. OpenBao documents several
non-audited paths, including initialization, seal status, seal, unseal, leader,
health, and Raft bootstrap or join paths. `sys/metrics`, `sys/pprof/*`, and
`sys/in-flight-req` also bypass audit when listener settings allow
unauthenticated access.

Audit logs still contain sensitive metadata. OpenBao HMACs most strings in
requests and responses, but non-string JSON values such as integers and
booleans pass through in plaintext. Restrict audit-log access even when
`log_raw` is disabled.

## Platform signals

Platform signals explain the environment around OpenBao. Use Kubernetes,
systemd, container runtime, filesystem, and network signals to answer questions
that OpenBao metrics cannot answer alone.

Use platform signals when you need to know whether:

- The pod or host restarted.
- The service discovery target disappeared.
- A volume that stores audit files is full or detached.
- A network policy blocks Prometheus, Alloy, or Grafana.
- A certificate or Secret rollout changed the scrape path.

Do not use platform health as a substitute for OpenBao health. A running pod
can still be sealed, leaderless, unable to write audit logs, or unable to serve
the metrics endpoint.

## Dashboards

Dashboards are interpretation views. They help you compare signals at a glance,
but they do not replace the source data.

Use dashboards to decide where to investigate next. Use metrics, logs, audit
records, and platform events as the evidence behind the decision.

## Common mistakes

- Treating audit logs and completed request logs as the same signal.
- Enabling completed request logs permanently without a retention and access
  decision.
- Sending audit logs to a broad operational log tenant.
- Using health endpoints as evidence that audit logging works.
- Reading dashboard panels as proof without checking the source query and
  scrape profile.
- Treating a quiet audit stream as healthy without a canary or collector-health
  signal.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents telemetry metric types, `usage_gauge_period`, audit-device behavior, unaudited paths, and completed request logging. |
| Observed fixture behavior | The Docker Compose fixture separates operational logs, audit logs, and Prometheus metrics so dashboards can query each signal independently. |
| Design decision | This project treats audit logs as security records, keeps them in a dedicated Loki stream, and uses audit canaries to distinguish quiet clusters from broken audit pipelines. |
| To validate | Retention, access control, SIEM forwarding, and platform-event coverage in your deployment environment. |

## Related pages

- Use [OpenBao observability model](./openbao-observability-model.md) to see
  how source signals become dashboards and alerts.
- Use [High-cardinality and label safety](./high-cardinality-and-label-safety.md)
  before you promote any log or metric field into a label.
- Use [Audit logs as security records](./audit-logs-as-security-records.md) to
  understand audit-log access, retention, and canary design.
- Use [Token and lease observability](./token-and-lease-observability.md) to
  separate inventory, rate, latency, and audit context.
- Use [Understanding OpenBao metrics](../metrics/understanding-openbao-metrics.md)
  when you need metric types, source prefixes, labels, and recording rules.
- Use [Understanding OpenBao logs](../logging/understanding-openbao-logs.md)
  when you need to choose between operational, request, audit, and archive
  streams.
- Use [OpenBao overview dashboard](../dashboards/overview-dashboard.md) to
  see how metrics, logs, and audit logs appear in one triage view.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  configure repeatable audit streams.
- Use [Audit canary missing](../runbooks/audit-canary-missing.md) when the
  canary-backed audit alert fires.
- Use [Audit request and response failures](../runbooks/audit-request-response-failures.md)
  when OpenBao reports audit write failures.

Source: OpenBao documents metric types and high-cardinality gauge behavior in
the [OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents audit behavior, unaudited paths, and HMAC limits in the
[OpenBao audit device documentation][openbao-audit]. OpenBao documents
completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
