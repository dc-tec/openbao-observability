# Audit canary missing

Use this runbook when the `OpenBaoAuditCanaryMissing` alert fires because Loki
has not received the expected audit canary request. The canary proves that
OpenBao writes audit events, the collector reads them, and Loki stores them
under the `openbao.audit` stream.

## Before you begin

- Get access to Loki or the log backend that evaluates the alert.
- Get access to the OpenBao token, policy, and secret path used by the canary.
- Get access to the collector that tails or receives OpenBao audit logs.
- Get security approval before changing audit device configuration.

> [!WARNING]
> Audit logs can contain sensitive request and response metadata. Do not paste
> raw audit log lines into tickets, chat, or public logs.

## Confirm the canary gap

1. Query the canary audit path for the alert window.

   ```logql
   count_over_time({log_stream="openbao.audit"} | json request_path="request.path" | request_path="secret/data/observability/audit-canary" [15m])
   ```

2. Query the full audit stream for the same window.

   ```logql
   count_over_time({log_stream="openbao.audit"}[15m])
   ```

3. If the full stream has events but the canary path is missing, investigate
   the canary token, policy, secret path, and scheduler.

4. If the full stream is also missing, use
   [Audit log stream missing](./audit-log-stream-missing.md) to inspect the
   collector and Loki ingestion path.

## Check the canary request

1. Confirm that the canary path exists.

   ```shell
   bao read secret/data/observability/audit-canary
   ```

2. Confirm that the canary policy grants only the expected read capability.

   ```shell
   bao policy read audit-canary
   ```

   Expected policy:

   ```hcl
   path "secret/data/observability/audit-canary" {
     capabilities = ["read"]
   }
   ```

3. Confirm that the canary token is valid.

   ```shell
   bao token lookup <audit_canary_token>
   ```

4. Run the canary request with the canary token.

   ```shell
   BAO_TOKEN=<audit_canary_token> bao read secret/data/observability/audit-canary
   ```

5. If the request fails, rotate or recreate the canary token with only the
   `audit-canary` policy.

## Check the scheduler

1. Confirm that the canary job or container is running.

2. Check the scheduler logs for OpenBao connection errors, token expiration,
   permission errors, and DNS failures.

3. Confirm that the scheduler interval is shorter than the alert window. The
   Docker Compose reference stack uses a 60-second interval and a 15-minute
   alert window.

4. Restart the scheduler through your normal deployment process.

## Verify the result

1. Run the canary request.

   ```shell
   BAO_TOKEN=<audit_canary_token> bao read secret/data/observability/audit-canary
   ```

2. Confirm that Loki receives the canary event.

   ```logql
   count_over_time({log_stream="openbao.audit"} | json request_path="request.path" | request_path="secret/data/observability/audit-canary" [5m])
   ```

3. Confirm that audit failure counters are not increasing.

   ```promql
   sum(increase(${p}_audit_log_request_failure[5m]))
   sum(increase(${p}_audit_log_response_failure[5m]))
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

4. Wait for the alert window to pass and confirm that
   `OpenBaoAuditCanaryMissing` resolves.

## Troubleshooting

### The canary request works but Loki has no event

Check audit device health, collector file permissions, collector positions,
Loki write errors, and stream labels.

### The canary token is expired or revoked

Create a new token with only the `audit-canary` policy and update the scheduler
secret.

### The canary path was deleted

Recreate the canary secret at `secret/data/observability/audit-canary`. Do not
store production secrets in this path.

## What's next

- Use [Audit log stream missing](./audit-log-stream-missing.md) when the full
  audit stream is missing.
- Use [Audit request and response failures](./audit-request-response-failures.md)
  when OpenBao reports audit write failures.

Source: OpenBao documents audit devices and audit log sensitivity in the
[OpenBao audit device documentation][openbao-audit].

[openbao-audit]: https://openbao.org/docs/audit/
