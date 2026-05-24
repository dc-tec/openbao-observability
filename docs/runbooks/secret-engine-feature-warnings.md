# Secret engine feature warnings

Use this runbook when a PKI or database secrets engine warning fires for
OpenBao. These alerts point to failed or unusually slow feature-specific
operations and need correlation with audit logs, operational logs, and the
external system behind the secret engine.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get access to OpenBao operational logs and audit logs.
- Get OpenBao CLI access with permission to inspect the affected secret engine.
- Get access to the external PKI or database platform if the alert points to a
  backend dependency.
- Get approval from the affected secret engine owner before you change roles,
  issuers, root credentials, leases, or mount configuration.

> [!WARNING]
> Do not rotate root credentials, revoke certificates, revoke leases, or change
> issuer configuration only to clear an alert. These actions can affect active
> workloads and must follow your local change or incident process.

## Confirm the warning

1. Check which warning fired.

   ```promql
   ALERTS{alertstate="firing", alertname=~"OpenBao(PKI|DatabaseCredential).*"}
   ```

2. Open the `OpenBao secret engines and mounts` dashboard.

   Open the `OpenBao database secrets` dashboard when a database warning
   fires.

3. Check whether the warning correlates with request latency, storage latency,
   audit failures, or HA/Raft alerts.

   ```promql
   openbao:core_handle_request:avg5m
   openbao:barrier_get:avg5m
   openbao:audit_log_request_failure:increase5m
   openbao:autopilot_healthy:max
   ```

4. Check operational logs around the alert window.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(pki|database|plugin|lease|revoke|issuer|certificate|connection|timeout|error|failed)"
   ```

## Investigate PKI warnings

1. Check PKI failure counters.

   ```promql
   openbao:pki_issue_failure:increase15m
   openbao:pki_revoke_failure:increase15m
   ```

2. Check PKI operation rate and latency.

   ```promql
   openbao:pki_issue:rate5m
   openbao:pki_revoke:rate5m
   openbao:pki_issue:avg5m
   openbao:pki_revoke:avg5m
   ```

3. Check audited PKI requests.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path", audit_error="error" | request_path=~"pki/(roles|issue|issuer|root|cert|tidy|revoke).*"
   ```

4. Inspect PKI mount configuration and issuer state.

   ```shell
   bao secrets list -detailed -address=<openbao_address>
   bao read -address=<openbao_address> pki/cert/ca
   bao list -address=<openbao_address> pki/roles
   ```

   - `<openbao_address>`: OpenBao API address for a reachable active node.

5. If certificate issue failures affect a specific role, inspect that role
   before you change issuer or mount-level configuration.

   ```shell
   bao read -address=<openbao_address> pki/roles/<role_name>
   ```

## Investigate database warnings

1. Check database operation failure counters.

   ```promql
   openbao:database_initialize_error:increase15m
   openbao:database_close_error:increase15m
   openbao:database_new_user_error:increase15m
   openbao:database_update_user_error:increase15m
   openbao:database_delete_user_error:increase15m
   ```

2. Check database credential operation rates and latency.

   ```promql
   openbao:database_new_user:rate5m
   openbao:database_update_user:rate5m
   openbao:database_delete_user:rate5m
   openbao:database_new_user:avg5m
   openbao:database_update_user:avg5m
   openbao:database_delete_user:avg5m
   openbao:database_close:avg5m
   ```

3. Check dynamic secret lease creation by engine.

   ```promql
   openbao:secret_lease_creation_by_engine:increase15m
   ```

4. Check audited database secrets engine requests.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path", audit_error="error" | request_path=~"database/(config|roles|creds|static-roles|static-creds|rotate-root|rotate-role).*"
   ```

5. Inspect database secrets engine configuration and roles.

   ```shell
   bao secrets list -detailed -address=<openbao_address>
   bao read -address=<openbao_address> database/config/<connection_name>
   bao read -address=<openbao_address> database/roles/<role_name>
   ```

6. Check the external database directly for connection limits, authentication
   failures, lock waits, permission errors, or slow credential-management
   statements.

## Restore the baseline

1. If failures correlate with external backend errors, restore the external
   backend before you change OpenBao configuration.

2. If failures started after a role, issuer, policy, plugin, or mount change,
   roll back or repair that change with the owner.

3. If database revocation fails, identify affected leases before you revoke or
   tidy lease state.

   ```shell
   bao list -address=<openbao_address> sys/leases/lookup/database/creds/<role_name>/
   ```

4. If PKI issue latency rises during expected high certificate volume, record
   the new baseline and expected duration in the change record.

5. If PKI revoke latency rises with storage or Raft symptoms, use the HA/Raft
   runbook before you tune PKI settings.

## Verify the result

1. Confirm that failure counters stop increasing.

   ```promql
   openbao:pki_issue_failure:increase15m
   openbao:pki_revoke_failure:increase15m
   openbao:database_new_user_error:increase15m
   openbao:database_update_user_error:increase15m
   openbao:database_delete_user_error:increase15m
   openbao:database_close_error:increase15m
   ```

2. Confirm that operation latency returns toward baseline.

   ```promql
   openbao:pki_issue:avg5m
   openbao:pki_revoke:avg5m
   openbao:database_new_user:avg5m
   openbao:database_update_user:avg5m
   openbao:database_delete_user:avg5m
   openbao:database_close:avg5m
   ```

3. Confirm that operational logs no longer show correlated backend or plugin
   errors.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(pki|database|plugin|lease|revoke)" |~ "(?i)(error|failed|timeout|denied)"
   ```

4. Wait for the alert window to pass and confirm that the warning resolves.

## Troubleshooting

### The alert fires with no dashboard data

Confirm that generated recording rules are loaded and that Prometheus scrapes
OpenBao source metrics with the expected `vault_*` or `openbao_*` prefix.

### Failure counters are empty

The failure metrics are optional and only appear after OpenBao emits the
underlying source counter. Check audit and operational logs to confirm whether
the alert came from a stale recording rule or a now-resolved failure.

### Latency is high but operations still succeed

Treat the warning as early pressure. Check storage, Raft, external database,
and client workload changes before you change secret engine configuration.

## What's next

- Use [OpenBao secret engines and mounts dashboard](../dashboards/secret-engines-mounts.md)
  to inspect feature metrics and audit context together.
- Use [OpenBao database secrets dashboard](../dashboards/database-secrets.md)
  to inspect database operation rates, latency, failures, leases, and audit
  streams together.
- Use [Irrevocable leases present](./irrevocable-leases.md) when database
  credential revocation leaves leases behind.
- Use [OpenBao Raft and Autopilot health](./raft-autopilot-health.md) when
  feature latency correlates with storage or Raft symptoms.

Source: OpenBao documents telemetry metric behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents database secrets engine behavior in the
[OpenBao database secrets engine documentation][openbao-database]. OpenBao
documents PKI behavior in the [OpenBao PKI documentation][openbao-pki].

[openbao-database]: https://openbao.org/docs/secrets/databases/
[openbao-pki]: https://openbao.org/docs/secrets/pki/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
