# Security audit detections

Use this runbook when a security detection alert fires from OpenBao audit logs
or completed request logs. These alerts identify sensitive activity that needs
change validation and security review, not automatic proof of compromise.

## Before you begin

- Get access to Loki or the log backend that evaluates the alert.
- Get access to the OpenBao audit overview and audit investigation dashboards.
- Get access to OpenBao change records and approved maintenance windows.
- Get security approval before changing audit devices, policies, auth methods,
  mounts, or plugins.

## Confirm the detection

1. Check which detection fired.

   ```logql
   {log_stream=~"openbao.audit|openbao.completed_requests"}
   ```

2. Open the `OpenBao audit overview` dashboard and check the security
   detection summary panels.

3. Open the `OpenBao audit investigation` dashboard and set filters for the
   alert window, request path, operation, and request ID.

4. Check whether the event matches an approved change record, automation run,
   deployment rollout, or troubleshooting window.

## Investigate audit configuration activity

1. Query audit configuration changes.

   ```logql
   {log_stream="openbao.audit"} | json operation="request.operation", request_id="request.id", request_path="request.path" | operation=~"create|update|delete" | request_path=~"sys/audit(/.*)?"
   ```

2. Confirm the current audit devices.

   ```shell
   bao audit list -detailed -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable active node.

3. Confirm that at least one approved audit device writes successfully.

4. If an audit device was disabled, removed, or changed outside an approved
   window, treat the event as a security incident until reviewed.

## Investigate privileged configuration changes

1. Query policy, auth method, and mount configuration changes.

   ```logql
   {log_stream="openbao.audit"} | json operation="request.operation", request_id="request.id", request_path="request.path" | operation=~"create|update|delete" | request_path=~"sys/(policies|auth|mounts)(/.*)?"
   ```

2. Check the current policies.

   ```shell
   bao policy list -address=<openbao_address>
   ```

3. Check enabled auth methods and secret engines.

   ```shell
   bao auth list -detailed -address=<openbao_address>
   bao secrets list -detailed -address=<openbao_address>
   ```

4. Confirm that each change has an owner and approval record.

## Investigate high-risk system paths

1. Query high-risk system path activity.

   ```logql
   {log_stream="openbao.audit"} | json request_id="request.id", request_path="request.path" | request_path=~"sys/(raw|plugins|storage/raft|rotate)(/.*)?"
   ```

2. Treat `sys/raw` activity as especially sensitive. Raw storage access can
   bypass normal secret engine boundaries.

3. Treat plugin, Raft storage, and rotation activity as operationally
   sensitive. Confirm that the event aligns with an approved operator action.

4. Preserve the request ID, timestamp, node, and nearby request and response
   entries for incident review.

## Investigate permission denied bursts

1. Query permission denied responses.

   ```logql
   {log_stream="openbao.audit"} | json audit_type="type", audit_error="error", request_id="request.id", request_path="request.path" | audit_type="response" | audit_error=~"(?s).*permission denied.*"
   ```

2. Use the request path filter to identify whether the denials cluster around
   auth, secret engine, policy, or system paths.

3. Compare the detection with expected application deployments, user activity,
   password spray tests, migration work, or automation failures.

4. If the burst targets privileged paths or follows a policy change, escalate
   to security review.

## Restore the baseline

1. Revert unauthorized policy, auth method, mount, plugin, or audit-device
   changes only through your incident process.

2. Restore the approved audit device configuration before making broad
   application or dashboard changes.

3. Review affected audit entries in the durable archive or SIEM when your
   deployment has one.

4. Keep Loki dashboard access restricted while the investigation is active.

## Verify the result

1. Confirm that the detection-specific query no longer increases outside the
   approved activity window.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"sys/(audit|policies|auth|mounts|raw|plugins|storage/raft|rotate).*"
   ```

2. Confirm that audit logs still arrive for the canary request.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path" | request_path="secret/data/observability/audit-canary"
   ```

3. Wait for the alert window to pass and confirm that the security detection
   alert resolves.

## Troubleshooting

### The activity was expected

Attach the approved change record, deployment run, or troubleshooting window to
the alert response. Do not remove the detection from the contract to suppress
expected maintenance noise.

### The request path is not enough context

Use request ID drilldown in the audit investigation dashboard. Compare request
and response entries with operational logs and change records.

### Permission denied responses are normal for an app

Fix the application policy or request path if the denials are recurring. Use a
silence only for an approved migration or test window.

## What's next

- Use [OpenBao audit overview dashboard](../dashboards/audit-overview.md) for
  security detection summary panels.
- Use [OpenBao audit investigation dashboard](../dashboards/audit-investigation.md)
  for request ID, path, operation, and node drilldown.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  to understand audit-log sensitivity and limitations.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add request, identity, token, or path fields to any label set.
- Use [Completed request logging enabled](./completed-request-logging-enabled.md)
  when the completed request logging posture alert fires.
- Use [Debug logging enabled](./debug-logging-enabled.md) when debug or trace
  logging is active with completed request logging.

Source: OpenBao documents audit-device behavior, unaudited paths, and HMAC
limits in the [OpenBao audit device documentation][openbao-audit]. OpenBao
documents completed request logging in the
[OpenBao completed request logging documentation][openbao-log-requests].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-log-requests]: https://openbao.org/docs/configuration/log-requests-level/
