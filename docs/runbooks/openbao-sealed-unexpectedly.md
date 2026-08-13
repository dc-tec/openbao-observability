# OpenBao sealed unexpectedly

Use this runbook when the `OpenBaoSealedUnexpectedly` alert fires because a
scraped OpenBao node reports `core_unsealed` as `0`. The steps help you confirm
whether the seal is planned, restore service through the approved unseal path,
and verify audit visibility after recovery.

## Before you begin

- Get access to the OpenBao node or pod that reports sealed.
- Get the approved unseal procedure for the deployment.
- Get access to the seal backend, such as the KMS, HSM, or key shares used by
  the cluster.
- Get access to operational logs for the affected node.

> [!WARNING]
> Never paste unseal keys, recovery keys, or root tokens into tickets, chat, or
> shared terminals. Follow your local break-glass process.

## Confirm seal state

1. Check seal and HA status with the OpenBao CLI.

   ```shell
   bao status -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for the affected node, including
     scheme and port.

2. Query the unauthenticated seal status endpoint.

   ```shell
   curl -fsS http://<openbao_address>/v1/sys/seal-status
   ```

   - `<openbao_address>`: OpenBao API address for the affected node.

3. Repeat the check for each node in the cluster. A single sealed standby has a
   different impact than all nodes sealed.

4. Check whether maintenance, a rolling restart, or a controlled seal operation
   explains the alert.

## Identify the cause

1. Inspect operational logs around the first alert timestamp.

   ```shell
   journalctl -u openbao --since <incident_start>
   ```

   - `<incident_start>`: Time shortly before the alert first fired.

2. Check the seal backend health. For auto-unseal, confirm that OpenBao can
   reach the configured KMS or HSM and that credentials, IAM permissions, or
   network paths have not changed.

3. Check storage health. Storage failures can leave nodes unable to complete
   startup and unseal.

4. Check recent node restarts, configuration changes, image changes, and
   platform events.

## Restore service

1. If the seal was planned, confirm the maintenance window and silence the alert
   only for the planned duration.

2. If the node uses Shamir unseal, run the approved unseal process on the
   affected node.

   ```shell
   bao operator unseal -address=<openbao_address> <unseal_key_share>
   ```

   - `<openbao_address>`: OpenBao API address for the sealed node.
   - `<unseal_key_share>`: One unseal key share from your approved key custody
     process.

3. If the node uses auto-unseal, restore the seal backend first. Restart the
   OpenBao process only after you have confirmed that the seal backend and
   storage backend are reachable.

4. Avoid repeated blind restarts. Repeated restarts can hide the root cause and
   make quorum or storage problems harder to diagnose.

## Verify the result

1. Confirm that the node is unsealed.

   ```shell
   bao status -address=<openbao_address>
   ```

   - `<openbao_address>`: OpenBao API address for the recovered node.

2. Confirm that metrics show the node as unsealed.

   ```promql
   openbao:core_unsealed:min{
     cluster="<cluster>",
     kubernetes_namespace="<kubernetes_namespace>",
     scrape_profile="<scrape_profile>"
   }
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

3. Confirm that clients can complete a permitted read or write.

4. Confirm that audit logs still arrive after recovery. Seal and unseal paths
   are not audit paths, so use a permitted audited request for this check.

## Troubleshooting

### The node seals again after unseal

Check the seal backend, storage backend, and operational logs before another
unseal attempt. Repeated reseal usually means OpenBao is losing a dependency
after startup.

### All nodes are sealed

Treat the incident as a cluster outage. Restore the seal backend and storage
backend before you unseal nodes. Escalate through the production break-glass
process if keys or recovery credentials are unavailable.

### The alert remains active after unseal

Check whether Prometheus still scrapes a stale or sealed target. Then use
[OpenBao metrics scrape failing](./openbao-metrics-scrape-failing.md) to
restore target health.

## Related pages

- Use [No active OpenBao leader](./no-active-openbao-leader.md) if nodes are
  unsealed but no active node is elected.
- Use [Audit log stream missing](./audit-log-stream-missing.md) if audit logs
  stop arriving after recovery.

Source: OpenBao documents the unauthenticated seal status endpoint in the
[OpenBao seal status API documentation][openbao-seal-status]. OpenBao documents
non-audited seal and health paths in the
[OpenBao audit device documentation][openbao-audit].

[openbao-audit]: https://openbao.org/docs/audit/
[openbao-seal-status]: https://openbao.org/api-docs/system/seal-status/
