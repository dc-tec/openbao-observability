# Audit archive reference design

Use this explainer to design a durable audit archive path for OpenBao. It is
for platform, security, and compliance teams that need audit evidence beyond
short-term dashboard exploration.

## Why this matters

OpenBao audit logs are security records. They show request and response activity
for audited paths, and OpenBao can fail or hang requests when audit devices
cannot write.

Loki, Grafana dashboards, and audit-investigation queries help you explore
recent activity. They do not, by themselves, prove that audit records were
preserved for the required retention period or protected against tampering.

The archive path answers different questions:

- Did each expected audit record reach a durable backend?
- Can you prove retention, deletion, and legal-hold behavior?
- Can you recover or export records during an investigation?
- Can you detect delivery gaps before they become evidence gaps?

## Reference model

Keep audit exploration and audit archive as separate paths.

```text
OpenBao audit devices
  -> local audit file
  -> restricted collector or security shipper
       -> openbao.audit for short-term exploration
       -> openbao.audit_archive for durable evidence delivery
  -> SIEM, immutable object store, security data lake, or WORM archive
```

The local audit file gives you a replay source when the downstream archive path
has a temporary outage. The archive delivery path gives you long-term retention,
tamper-resistance controls, and security-team ownership.

## Recommended baseline

For production, use this baseline:

1. Configure a local file audit device on dedicated storage.
2. Configure at least one independent audit path or collector delivery path for
   archive delivery.
3. Send short-term exploration records to a restricted log backend such as Loki.
4. Send durable evidence records to an approved archive path.
5. Monitor both OpenBao audit-device health and archive delivery health.

Do not use a network-only audit sink as the only audit device. OpenBao documents
that the HTTP audit device is synchronous and does not retry, and that blocked
audit devices can prevent OpenBao from completing requests.

Source: OpenBao documents multiple audit devices and blocking audit behavior in
the [OpenBao audit device documentation][openbao-audit]. OpenBao documents file
and HTTP audit-device behavior in the
[OpenBao file audit device documentation][openbao-file-audit] and
[OpenBao HTTP audit device documentation][openbao-http-audit].

## Archive patterns

Choose an archive pattern based on evidence requirements, failure behavior, and
the systems your organization already operates.

| Pattern | Use when | Watch |
| ------- | -------- | ----- |
| File audit device plus shipper to immutable object storage. | You need a replayable local source and storage-level retention controls. | Monitor disk space, file rotation, shipper lag, object-store write failures, and lifecycle policy. |
| File audit device plus security collector to SIEM. | Your security team already investigates and retains evidence in a SIEM. | Require delivery acknowledgement, dead-letter handling, retention policy, and export process. |
| HTTP audit device to an internal archive gateway. | You need central routing at request time and have tested gateway behavior. | Use it with another audit device. Test slow, failed, and partial gateway responses before production. |
| Syslog or socket audit device to a local security agent. | You have host-level security logging standards or legacy SIEM ingestion. | Avoid UDP-only designs for evidence. Test large records, local agent outages, and TCP backpressure. |
| Container stdout collected by the platform. | You run a local demo or short-lived test environment. | Treat it as a demo path unless the platform provides approved retention, access control, and archive delivery. |

OpenBao can write to several audit device types, but the device type does not
make the destination archive-grade. The archive guarantee comes from delivery
confirmation, retention controls, access control, integrity controls, and
restore or export testing.

## Archive backend choices

The following backend choices are common. Validate the exact behavior in your
cloud, region, account policy, and compliance context.

| Backend | Useful controls | Design checks |
| ------- | --------------- | ------------- |
| AWS S3 with Object Lock. | WORM retention, governance mode, compliance mode, version-level retention, and legal holds. | Enable Object Lock before relying on it. Verify bucket policy, versioning, retention mode, legal hold, and lifecycle behavior. |
| Google Cloud Storage with Bucket Lock. | Bucket retention policies and locked retention policies that prevent reducing or removing retention. | Treat locking as irreversible. Verify retention duration, object versioning needs, lifecycle rules, and organization policy. |
| Azure Blob Storage immutable storage. | Time-based retention policies, legal holds, container-level policies, and version-level policies. | Verify versioning, policy scope, locked policy behavior, unsupported account features, and failover behavior. |
| SIEM or security data lake. | Correlation, detection, retention policy, case workflow, and evidence export. | Verify ingestion acknowledgement, replay, dead-letter handling, access model, deletion controls, and legal-hold process. |
| S3-compatible object storage. | Often supports object lock or retention-like controls. | Validate the exact WORM semantics. Do not assume AWS S3 Object Lock behavior from an S3-compatible API alone. |

Source: AWS documents S3 Object Lock in the
[Amazon S3 Object Lock documentation][aws-object-lock]. Google Cloud documents
Bucket Lock in the [Cloud Storage Bucket Lock documentation][gcs-bucket-lock].
Azure documents immutable blob storage in the
[Azure immutable storage documentation][azure-immutable-storage].

## Delivery contract

The archive path should expose operational signals that can drive an
`OpenBaoAuditArchiveDegraded` alert.

Track these signals where your collector, gateway, SIEM, or object-store writer
can emit them:

- Last successful archive delivery timestamp.
- Archive delivery success and failure count.
- Delivery lag from audit-file write time to archive acceptance time.
- Local backlog size or queued batch count.
- Rejected records and dead-letter count.
- Retry count and oldest retry age.
- Object-store or SIEM write errors.
- Audit canary arrival in the archive backend.

Use these reference metric names when you want to use the generated
`OpenBaoAuditArchiveDegraded` alert:

| Metric | Meaning |
| ------ | ------- |
| `openbao_audit_archive_enabled` | `1` when this environment expects durable audit archive delivery. |
| `openbao_audit_archive_delivery_success` | `1` when the archive path is healthy and `0` when delivery is degraded. |
| `openbao_audit_archive_last_success_timestamp_seconds` | Unix timestamp for the last successful archive delivery or acknowledgement. |
| `openbao_audit_archive_delivery_failures_total` | Count of failed archive writes, rejected batches, or failed acknowledgements. |
| `openbao_audit_archive_dead_letter_records_total` | Count of records sent to a dead-letter path instead of the durable archive. |

Do not page on a backend-specific expression until you know what the signal
means. A queue depth metric, SIEM acknowledgement metric, and object-store write
metric each describe a different part of the archive path.

Use the [audit archive health example](../../examples/audit-archive-health/)
when you need a small exporter or a recording-rule mapping pattern for these
metrics.

## Failure modes to test

Test archive behavior before you declare the path production-ready.

| Failure mode | Expected behavior |
| ------------ | ----------------- |
| Local audit volume is full. | OpenBao audit failure metrics increase, requests fail or hang according to audit-device behavior, and operators page before evidence is lost. |
| Local audit file is rotated. | OpenBao receives `SIGHUP`, reopens the file, and the collector follows the new file without skipping records. |
| Collector loses read permission. | `openbao.audit` and archive delivery health fail while OpenBao audit-device metrics stay healthy. |
| Archive backend is unavailable. | Local audit file continues to accumulate records, backlog grows, and delivery alerts fire. |
| HTTP audit gateway is slow or down. | OpenBao impact is understood and documented before the device is used in production. |
| SIEM rejects records. | Records reach a dead-letter path with enough metadata to replay or investigate rejection. |
| Loki is unavailable. | Archive delivery continues if Loki is only the exploration backend. |
| Archive succeeds but Loki is quiet. | Investigation dashboards show a gap, but security evidence remains available through the archive path. |
| Clock skew appears between nodes and backend. | Archive queries can still correlate by request ID, node, and ingestion timestamp. |
| Records are replayed after outage. | The backend handles duplicates or the investigation process documents duplicate handling. |

## Access and retention controls

Separate archive access from dashboard exploration access.

Use these boundaries:

- Grant `openbao.audit` access only to approved security responders.
- Grant `openbao.audit_archive` access only through the evidence-handling
  process.
- Keep archive writer credentials separate from OpenBao runtime credentials.
- Keep delete, retention-policy, and legal-hold permissions separate from read
  permissions.
- Record retention, legal hold, and exception decisions outside the dashboard
  repository.

OpenBao HMACs many sensitive strings in audit records, but audit records still
contain sensitive metadata. Treat archive readers as privileged security users.

## Validation checklist

Use this checklist before you rely on the archive path:

- OpenBao has at least one local file audit device on dedicated storage.
- Production does not rely on one network-only audit sink.
- The collector can replay from the local audit file after an outage.
- The archive backend confirms successful writes or exposes a failure signal.
- The archive path has dead-letter or rejection handling.
- The archive path has documented retention and deletion behavior.
- Legal hold or equivalent preservation is tested when required.
- Access to archive records is separate from operational log access.
- Audit canary events reach the archive backend within the expected window.
- Restore, export, and investigation workflows are tested with sample records.

## Common mistakes

- Treating Loki retention as an audit archive policy.
- Sending only container stdout to a broad logging tenant.
- Using an HTTP audit device as the only production audit device.
- Designing archive retention without a tested export process.
- Giving the same role read access to operational logs and audit archives.
- Assuming object-store lifecycle rules are the same as immutability.
- Assuming a SIEM is archive-grade without acknowledgement and replay behavior.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents multiple audit devices, audit blocking behavior, file rotation behavior, synchronous HTTP audit behavior, and audit failure metrics. |
| Confirmed external behavior | AWS, Google Cloud, and Azure document storage-level immutability and retention controls. Loki documents operational retention through the Compactor. |
| Design decision | This project separates `openbao.audit` exploration from `openbao.audit_archive` evidence delivery and recommends a local file replay source for production. |
| To validate | SIEM acknowledgement, object-store retention settings, legal hold, dead-letter handling, replay behavior, and archive alert expressions in your environment. |

## What's next

- Use [Configure declarative audit devices](./declarative-audit.md) to
  configure repeatable audit devices.
- Use [Audit logs as security records](../concepts/audit-logs-as-security-records.md)
  to understand audit-log sensitivity and HMAC limits.
- Use [Log retention and access control](../logging/retention-and-access-control.md)
  to separate operational, audit, and archive retention.
- Use [OpenBao audit investigation dashboard](../dashboards/audit-investigation.md)
  for short-term audit exploration.
- Use [Audit request and response failures](../runbooks/audit-request-response-failures.md)
  when OpenBao reports audit-device failures.
- Use [Audit canary missing](../runbooks/audit-canary-missing.md) when the
  canary-backed audit stream alert fires.
- Use [Audit archive degraded](../runbooks/audit-archive-degraded.md) when
  archive delivery health fails.
- Use the [audit archive health example](../../examples/audit-archive-health/)
  to publish the reference metrics for the generated alert.

Source: General log-management guidance in
[NIST SP 800-92][nist-sp-800-92] supports separate log-management
infrastructure, retention planning, protection, and archive practices. Loki
documents operational retention in the
[Grafana Loki retention documentation][loki-retention]. Grafana Alloy
documents file tailing positions in the
[Alloy file source documentation][alloy-file-source].

[alloy-file-source]: https://grafana.com/docs/alloy/latest/reference/components/loki.source.file/
[aws-object-lock]: https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html
[azure-immutable-storage]: https://learn.microsoft.com/en-us/azure/storage/blobs/immutable-storage-overview
[gcs-bucket-lock]: https://cloud.google.com/storage/docs/bucket-lock
[loki-retention]: https://grafana.com/docs/loki/latest/operations/storage/retention/
[nist-sp-800-92]: https://csrc.nist.gov/pubs/sp/800/92/final
[openbao-audit]: https://openbao.org/docs/audit/
[openbao-file-audit]: https://openbao.org/docs/audit/file/
[openbao-http-audit]: https://openbao.org/docs/audit/http/
