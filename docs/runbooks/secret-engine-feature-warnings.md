# Secret engine feature warnings

Use this runbook when a PKI, Transit, or database secrets engine warning fires
for OpenBao. These alerts point to failed or unusually slow feature-specific
operations and need correlation with audit logs, operational logs, and the
backend or application context behind the secret engine.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get access to OpenBao operational logs and audit logs.
- Get OpenBao CLI access with permission to inspect the affected secret engine.
- Get access to the external PKI, database, or application platform if the
  alert points to a backend dependency or client workload.
- Get approval from the affected secret engine owner before you change roles,
  issuers, root credentials, leases, or mount configuration.

> [!WARNING]
> Do not rotate root credentials, revoke certificates, revoke leases, or change
> issuer configuration only to clear an alert. These actions can affect active
> workloads and must follow your local change or incident process.

## Confirm the warning

1. Check which warning fired.

   ```promql
   ALERTS{alertstate="firing", alertname=~"OpenBao(PKI|Transit|DatabaseCredential).*"}
   ```

2. Open the `OpenBao secret engines and mounts` dashboard.

   Open the `OpenBao PKI`, `OpenBao Transit`, or `OpenBao database secrets`
   dashboard when a feature-specific warning fires.

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
   {log_stream="openbao.operational"} |~ "(?i)(pki|transit|database|plugin|lease|revoke|issuer|certificate|connection|crypto|key|timeout|error|failed)"
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

## Investigate Transit warnings

1. Check audited Transit response errors.

   ```logql
   {log_stream="openbao.audit"} | json audit_type="type", request_path="request.path", audit_error="error" | audit_type="response" | audit_error!="" | request_path=~"transit/(keys|encrypt|decrypt|rewrap|sign|verify|hmac|random|hash|datakey).*"
   ```

2. Check denied Transit requests.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path", audit_error="error" | audit_error=~"(?s).*permission denied.*" | request_path=~"transit/(keys|encrypt|decrypt|rewrap|sign|verify|hmac|random|hash|datakey).*"
   ```

3. Check whether errors affect key management or cryptographic operations.

   ```logql
   {log_stream="openbao.audit"} | json audit_type="type", request_path="request.path", request_id="request.id" | audit_type="request" | request_path=~"transit/(keys|encrypt|decrypt|rewrap|sign|verify|hmac|random|hash|datakey).*"
   ```

4. Inspect Transit mount configuration and key metadata before you change key
   policy, deletion settings, or rotation settings.

   ```shell
   bao secrets list -detailed -address=<openbao_address>
   bao list -address=<openbao_address> transit/keys
   bao read -address=<openbao_address> transit/keys/<key_name>
   ```

5. If errors affect decrypt, verify, or rewrap operations, check for recent key
   rotations, key version changes, policy changes, or application release
   changes before you rotate again.

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

2. If failures started after a role, issuer, Transit key, policy, plugin, or
   mount change, roll back or repair that change with the owner.

3. If database revocation fails, identify affected leases before you revoke or
   tidy lease state.

   ```shell
   bao list -address=<openbao_address> sys/leases/lookup/database/creds/<role_name>/
   ```

4. If PKI issue latency rises during expected high certificate volume, record
   the new baseline and expected duration in the change record.

5. If PKI revoke latency rises with storage or Raft symptoms, use the HA/Raft
   runbook before you tune PKI settings.

6. If Transit errors affect application decrypt, verify, or rewrap traffic,
   coordinate recovery with the application owner before you delete keys,
   change key versions, disable deletion protection, or change derived key
   settings.

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

2. Confirm that Transit audit response errors stop increasing.

   ```logql
   sum(count_over_time({log_stream="openbao.audit"} | json audit_type="type", request_path="request.path", audit_error="error" | audit_type="response" | audit_error!="" | request_path=~"transit/(keys|encrypt|decrypt|rewrap|sign|verify|hmac|random|hash|datakey).*" [5m]))
   ```

3. Confirm that operation latency returns toward baseline.

   ```promql
   openbao:pki_issue:avg5m
   openbao:pki_revoke:avg5m
   openbao:database_new_user:avg5m
   openbao:database_update_user:avg5m
   openbao:database_delete_user:avg5m
   openbao:database_close:avg5m
   ```

4. Confirm that operational logs no longer show correlated backend or plugin
   errors.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(pki|transit|database|plugin|lease|revoke|crypto|key)" |~ "(?i)(error|failed|timeout|denied)"
   ```

5. Wait for the alert window to pass and confirm that the warning resolves.

## Troubleshooting

### The alert fires with no dashboard data

Confirm that generated recording rules are loaded and that Prometheus scrapes
OpenBao source metrics with the expected `vault_*` or `openbao_*` prefix.

### Failure counters are empty

The failure metrics are optional and only appear after OpenBao emits the
underlying source counter. Check audit and operational logs to confirm whether
the alert came from a stale recording rule, a log-based Transit alert, or a
now-resolved failure.

### Transit alert does not match a custom mount

The generated Transit warning matches the default `transit` mount path. If your
deployment mounts Transit elsewhere, copy the alert and replace the
`transit/` path prefix with the approved mount path.

### Latency is high but operations still succeed

Treat the warning as early pressure. Check storage, Raft, external database,
and client workload changes before you change secret engine configuration.

## What's next

- Use [OpenBao secret engines and mounts dashboard](../dashboards/secret-engines-mounts.md)
  to inspect feature metrics and audit context together.
- Use [OpenBao PKI dashboard](../dashboards/pki.md) to inspect PKI operation
  metrics and certificate lifecycle audit streams together.
- Use [OpenBao Transit dashboard](../dashboards/transit.md) to inspect Transit
  key management, cryptographic operations, and response errors.
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
OpenBao documents Transit behavior in the
[OpenBao Transit documentation][openbao-transit].

[openbao-database]: https://openbao.org/docs/secrets/databases/
[openbao-pki]: https://openbao.org/docs/secrets/pki/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
[openbao-transit]: https://openbao.org/docs/secrets/transit/
