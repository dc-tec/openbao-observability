# Irrevocable leases present

Use this runbook when the `OpenBaoIrrevocableLeasesPresent` alert fires because
OpenBao reports one or more leases that it cannot automatically revoke. Treat
this as a revocation and cleanup risk until you identify the affected backend.

## Before you begin

- Get access to Prometheus or the metrics backend that evaluates the alert.
- Get OpenBao CLI access with permission to inspect leases and secret engines.
- Get access to OpenBao operational logs and audit logs.
- Get approval from the affected secret engine owner before revoking leases,
  running lease tidy, or changing mount configuration.

> [!WARNING]
> Do not force-delete leases to clear the alert unless the secret engine owner
> and security incident process approve it. Forced cleanup can leave external
> credentials valid outside OpenBao.

## Confirm the signal

1. Check the normalized recording rule.

   ```promql
   openbao:expire_num_irrevocable_leases:max
   ```

2. Check the raw source metric for your OpenBao metric prefix.

   ```promql
   max(${p}_expire_num_irrevocable_leases)
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

3. Open the `OpenBao token and lease lifecycle` dashboard and confirm whether
   lease count, token count, or lease operation latency changed around the same
   time.

## Find the affected lease area

1. Check recent lease management audit events.

   ```logql
   {log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"sys/leases/.*"
   ```

2. Check operational logs for lease, revocation, plugin, storage, or backend
   errors.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(lease|revoke|expiration|plugin|backend|storage)" |~ "(?i)(error|failed|timeout|denied)"
   ```

3. List enabled secret engines.

   ```shell
   bao secrets list -detailed -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for a reachable active node.

4. List lease prefixes only when you have the required `sudo` capability.

   ```shell
   bao list -address=<openbao_address> sys/leases/lookup/<lease_prefix>
   ```

   - `<lease_prefix>`: Lease prefix to inspect, for example
     `database/creds/readonly/`.

5. Look up individual leases only when the lease ID is approved for
   investigation.

   ```shell
   bao lease lookup -address=<openbao_address> <lease_id>
   ```

   - `<lease_id>`: Full lease ID to inspect.

## Restore revocation

1. If an external secret backend is unavailable, restore the backend before you
   revoke leases.

2. If a mount, role, plugin, or credential path changed, coordinate with the
   owner and restore enough configuration for OpenBao to revoke the lease
   cleanly.

3. Revoke only approved lease IDs.

   ```shell
   bao lease revoke -address=<openbao_address> <lease_id>
   ```

4. Avoid `revoke-prefix` for broad prefixes unless the owner approves the blast
   radius.

5. Run lease tidy only when upgrade notes, support guidance, or your local
   incident process calls for it.

## Verify the result

1. Confirm that irrevocable leases return to zero.

   ```promql
   openbao:expire_num_irrevocable_leases:max
   ```

2. Confirm that lease and revocation errors stop in operational logs.

   ```logql
   {log_stream="openbao.operational"} |~ "(?i)(lease|revoke|expiration)" |~ "(?i)(error|failed|timeout)"
   ```

3. Confirm that audit logs still arrive for permitted lease operations.

   ```logql
   count_over_time({log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"sys/leases/.*" [5m])
   ```

4. Wait for the alert window to pass and confirm that
   `OpenBaoIrrevocableLeasesPresent` resolves.

## Troubleshooting

### The value stays above zero after revocation

Confirm that you revoked the correct lease ID and that the affected backend is
healthy. Then wait for the next metrics scrape and rule evaluation.

### You cannot identify the lease prefix

Check recent mount and role changes. A removed or remounted secret engine can
make lease investigation harder and usually requires the engine owner.

### Revocation fails repeatedly

Restore the external backend or plugin first. If the external credential no
longer exists, record that evidence and follow your local security cleanup
process before forcing OpenBao-side lease cleanup.

## Related pages

- Use [Run the Docker Compose stack](../docker-compose.md) to inspect the
  `OpenBao token and lease lifecycle` dashboard locally.
- Use [Audit request and response failures](./audit-request-response-failures.md)
  if audit writes fail while you investigate leases.

Source: OpenBao documents lease lookup, revoke, prefix revoke, and tidy
behavior in the [OpenBao leases API documentation][openbao-leases]. OpenBao
documents metric types and high-cardinality gauge behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. OpenBao
documents `vault.expire.num_irrevocable_leases` in the
[OpenBao telemetry metrics reference][openbao-telemetry-all].

[openbao-leases]: https://openbao.org/api-docs/system/leases/
[openbao-telemetry-all]: https://openbao.org/docs/internals/telemetry/metrics/all/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
