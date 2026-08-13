# Configure declarative audit devices

Use this how-to to configure OpenBao audit devices from server configuration.
This profile gives you repeatable audit devices for HA OpenBao deployments and
keeps audit changes under normal configuration review.

## Before you begin

- Run OpenBao outside development mode.
- Get access to the OpenBao server configuration for every node.
- Choose the audit storage path and retention policy with your security team.
- Confirm that the OpenBao process can write to the audit path.
- Confirm that your log collector can read the audit path without broadening
  access to other OpenBao files.

> [!WARNING]
> Audit logs are security evidence. Do not configure `log_raw = "true"` for this
> reference profile. Raw audit logging can expose sensitive values that OpenBao
> normally HMACs.

## Choose the audit profile

| Profile | Use when | Audit device | Collector path | Caveat |
| ------- | -------- | ------------ | -------------- | ------ |
| Local demo | You run the Docker Compose stack or a disposable test cluster. | File audit device on local storage or `stdout`. | Alloy or container logs. | Not a compliance archive. |
| Kubernetes baseline | You run OpenBao on Kubernetes and need a repeatable audit stream. | File audit device on a mounted audit volume. | Restricted Alloy file tail or security collector. | Requires PVC, file permission, and retention planning. |
| Production Kubernetes | You need audit resilience and separate retention. | At least two audit devices, usually local file plus an independent path. | Restricted collector plus archive or SIEM path. | Test blocking and sink-failure behavior before rollout. |
| VM/systemd | You run OpenBao on hosts. | File audit device under `/var/log/openbao`. | Alloy file source, journald-adjacent collector, or security shipper. | Configure log rotation and send `SIGHUP` after rotation. |

Use declarative audit configuration as the baseline. OpenBao creates and
removes declarative audit devices on the active node during restart and
`SIGHUP` handling. The same configuration must exist on every server.

## Configure a file audit device

1. Add a file audit stanza to each OpenBao server configuration.

   ```hcl
   audit "file" "audit-file" {
     description = "Baseline audit file."

     options {
       file_path     = "/openbao/audit/audit.log"
       mode          = "0600"
       format        = "json"
       hmac_accessor = "true"
       log_raw       = "false"
     }
   }
   ```

2. Use the same audit path and stanza name on every OpenBao server.

3. Mount the audit path on dedicated storage when the deployment runs on
   Kubernetes.

4. Restrict file permissions so the OpenBao process can write the file and only
   the approved collector can read it.

5. Reload or restart OpenBao through your normal change process.

## Configure production redundancy

1. Keep a local file audit device as the primary path.

   ```hcl
   audit "file" "primary-file" {
     description = "Primary local audit file."

     options {
       file_path     = "/openbao/audit/audit.log"
       mode          = "0600"
       format        = "json"
       hmac_accessor = "true"
       log_raw       = "false"
     }
   }
   ```

2. Add a second audit path only after you test its failure behavior.

   ```hcl
   audit "file" "secondary-stdout" {
     description = "Secondary audit stream for independent collection."

     options {
       file_path     = "stdout"
       format        = "json"
       hmac_accessor = "true"
       log_raw       = "false"
     }
   }
   ```

3. Send one audit stream to a restricted exploration backend, such as Loki.

4. Send the production archive stream to your approved SIEM, object store, or
   immutable archive path.

Loki is useful for short-term exploration and dashboard correlation. Do not
treat Loki as the compliance archive unless your organization has explicitly
approved that design.

## Collect audit logs

1. Configure the collector to read only the audit file path.

2. Label the stream as `log_stream="openbao.audit"`.

3. Keep request identifiers, paths, entity IDs, token accessors, policies, and
   client addresses out of Loki labels. Parse those fields at query time.

4. Restrict the Grafana folder, Loki tenant, or equivalent backend access for
   audit dashboards.

5. Keep a separate archive path for long-term retention when your compliance
   requirements need it.

The Docker Compose stack in this repository demonstrates this pattern by
writing audit JSON lines to each OpenBao node and tailing them with Alloy under
the `openbao.audit` log stream.

## Configure an audit canary

1. Create a non-sensitive canary secret.

   ```shell
   bao kv put secret/observability/audit-canary status=ok purpose=audit-canary
   ```

2. Create a canary policy that can read only the canary path.

   ```hcl
   path "secret/data/observability/audit-canary" {
     capabilities = ["read"]
   }
   ```

3. Create a token for the canary scheduler with only the canary policy.

   ```shell
   bao token create -policy=audit-canary -no-default-policy
   ```

4. Run the canary request on a schedule that is shorter than the alert window.

   ```shell
   BAO_TOKEN=<audit_canary_token> bao read secret/data/observability/audit-canary
   ```

5. Alert on the absence of that specific audited path instead of treating every
   quiet audit stream as a critical incident.

   ```logql
   absent_over_time({log_stream="openbao.audit"} | json request_path="request.path" | request_path="secret/data/observability/audit-canary" [15m])
   ```

## Configure log rotation on hosts

1. Rotate file audit logs with your host log-rotation tool.

2. Send `SIGHUP` to OpenBao after each rotation so OpenBao closes and reopens
   the file.

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

3. Confirm that the collector follows the rotated file behavior you use.

## Verify the result

1. List audit devices.

   ```shell
   bao audit list -detailed -address=<openbao_address>
   ```

2. Confirm that the expected file audit device exists.

3. Run a permitted request against an approved audited path.

   ```shell
   bao read -address=<openbao_address> <approved_audited_path>
   ```

4. Confirm that the audit file receives request and response entries.

   ```shell
   stat /openbao/audit/audit.log
   ```

5. Confirm that the collector sends the stream to Loki.

   ```logql
   count_over_time({log_stream="openbao.audit"}[5m])
   ```

6. Confirm that OpenBao does not report audit write failures.

   ```promql
   sum(increase(${p}_audit_log_request_failure[5m]))
   sum(increase(${p}_audit_log_response_failure[5m]))
   ```

   - `${p}`: Metric prefix for your deployment. Use `vault` for the OpenBao
     default prefix or `openbao` when you configured
     `metrics_prefix = "openbao"`.

## Troubleshooting

### The audit device disappears after restart

Check that the same declarative audit stanza exists on every OpenBao server.
Also check that the stanza path does not duplicate an API-created audit device.

### The audit file stays empty

Check the audit path, file ownership, directory permissions, and mounted volume.
Then run a permitted audited request. Some health, seal, and leader paths are
not audit paths, so do not use them as the only verification signal.

### Requests hang after an audit change

Treat the change as an audit sink incident. Restore a known-good audit path
before removing or replacing a failing path, and get security approval before
you disable or move an audit device.

### Loki has no audit stream

Check the collector file target, read permissions, positions file, and labels.
The reference dashboards and alerts expect `log_stream="openbao.audit"`.

## Related pages

- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to choose the right signal for troubleshooting or investigation.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  to understand audit-log access, retention, and canary design.
- Use [Audit archive reference design](./audit-archive-reference-design.md) to
  design the durable evidence path for production audit records.
- Use [Log retention and access control](../logging/retention-and-access-control.md)
  before you choose retention periods and archive paths.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add audit fields to collector labels.
- Use [Run the Docker Compose stack](../docker-compose.md) to inspect the local
  audit pipeline.
- Use [Audit request and response failures](../runbooks/audit-request-response-failures.md)
  when OpenBao reports audit write failures.
- Use [Audit log stream missing](../runbooks/audit-log-stream-missing.md) when
  the log backend stops receiving audit entries.
- Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
  when you need to map raw audit metrics into normalized rules.

Source: OpenBao documents configuration-managed audit devices in the
[OpenBao declarative audit documentation][openbao-declarative-audit]. OpenBao
documents file audit behavior, log rotation, `stdout`, `mode`, and API
creation caveats in the
[OpenBao file audit device documentation][openbao-file-audit].

[openbao-declarative-audit]: https://openbao.org/docs/configuration/audit/
[openbao-file-audit]: https://openbao.org/docs/audit/file/
