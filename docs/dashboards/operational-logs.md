# OpenBao operational logs dashboard

Use this explainer to read the generated OpenBao operational logs dashboard. It
is for operators who need process-level troubleshooting signals from the
OpenBao server log stream.

## What this dashboard is for

Use the operational logs dashboard when you need to inspect OpenBao server
behavior across nodes.

The dashboard answers these questions:

- Does Loki receive operational logs from each OpenBao node?
- Are warning or error logs increasing?
- Are debug or trace logs enabled?
- Do Raft, storage, or Autopilot messages appear around an incident?
- Do seal or unseal messages appear unexpectedly?
- Which recent log entries need deeper inspection?

## What this dashboard is not for

Do not use operational logs as audit evidence. Operational logs describe
server behavior. Audit logs describe audited API request and response records.

Do not use this dashboard as a compliance archive. Loki is an exploration and
correlation backend unless your organization explicitly approves it for
long-term evidence retention.

## Required data source

The generated dashboard expects a Loki data source with UID `loki`. The panels
query operational logs collected with `log_stream="openbao.operational"` and a
bounded `node_id` label.

The generated dashboard exposes a Kubernetes namespace selector for
deployments where multiple OpenBao instances share the same observability
backend.

The dashboard does not need Prometheus for its current panels, even though the
contract includes the standard metrics data-source definition for consistency
with the dashboard generator.

## How to read log volume

The operational log volume panel counts OpenBao operational log entries by
node over five minutes. Use it to check collector presence and relative node
activity.

A flat line can mean the node is quiet, the collector stopped, labels changed,
or Loki ingestion failed. Use the operational log stream missing runbook when
the expected stream disappears.

## How to read warning and error logs

The warning and error panels count entries where the structured log level is
`warn` or `error`. Use them as triage signals.

Open a surrounding time window before you interpret a spike. A warning during
startup can have a different meaning than a warning during steady-state
request handling.

## How to read debug and trace logs

Healthy production profiles keep debug and trace volume at `0`. Nonzero volume
means OpenBao or a surrounding profile is emitting highly verbose logs.

Use the debug logging runbook when this panel stays nonzero outside a planned
troubleshooting window.

## How to read Raft and storage logs

The Raft and storage panels filter operational logs for Raft, storage, and
Autopilot terms. Use them with the HA/Raft dashboard when leadership,
replication, or peer health changes.

Raft-related log volume can increase during node joins, restarts, snapshots,
network problems, storage problems, or leadership changes. Check the change
window and platform events before you assign cause.

## How to read seal and unseal logs

Seal and unseal log volume points to lifecycle events. A planned restart with
auto unseal can produce expected entries. Unexpected entries deserve
investigation because they can explain traffic loss, scrape failures, or audit
stream gaps.

## Completed request logs

OpenBao completed request logging is separate from audit devices and is
disabled by default. If you enable it, OpenBao emits completed request logs at
the configured `log_requests_level` only when the main `log_level` includes
that level.

Use completed request logs for temporary troubleshooting. Do not treat them as
an audit-log replacement.

## Common mistakes

- Treating operational logs as security audit records.
- Leaving debug or trace logging enabled after troubleshooting.
- Assuming a quiet log stream proves the node is healthy.
- Searching for audit request IDs in the operational stream instead of the
  audit stream.
- Ignoring label changes when a Loki panel becomes empty.

## Known limitations

- The dashboard assumes `log_stream="openbao.operational"`.
- Log-level filters assume OpenBao structured log entries contain `@level`.
- The dashboard can only show logs that Loki retains.
- Completed request logs appear only when enabled in OpenBao configuration.
- Operational logs do not cover every audited API detail.

## What's next

- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to choose between operational logs and audit logs.
- Use [Understanding OpenBao logs](../logging/understanding-openbao-logs.md)
  to understand operational, completed request, audit, and archive streams.
- Use [Log retention and access control](../logging/retention-and-access-control.md)
  before you change retention or dashboard access.
- Use [OpenBao HA/Raft dashboard](./ha-raft.md) when Raft or Autopilot log
  panels show activity.
- Use [Operational log stream missing](../runbooks/operational-log-stream-missing.md)
  when Loki stops receiving operational logs.
- Use [Debug logging enabled](../runbooks/debug-logging-enabled.md) when debug
  or trace logs remain active.
- Use [Completed request logging enabled](../runbooks/completed-request-logging-enabled.md)
  when completed request logs appear outside an approved troubleshooting
  window.

Source: OpenBao documents completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests]. This
page describes the generated dashboard contract in
`contracts/dashboards/openbao-operational-logs.yaml`.

[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
