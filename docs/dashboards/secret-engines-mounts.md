# OpenBao secret engines and mounts dashboard

Use this explainer to read the generated OpenBao secret engines and mounts
dashboard. It is for operators who need restricted audit-log visibility into
mount lifecycle activity and common secret engine paths.

## What this dashboard is for

Use the secret engines and mounts dashboard when you need to inspect audited
activity for mounts, KV, database, Transit, PKI, and secret-engine response
errors. It also includes secret-engine metric signals where OpenBao exposes
safe aggregate data.

The dashboard answers these questions:

- Which secret engine events match the current filters?
- Did mount table reads or mount lifecycle changes occur?
- Did KV v1 or KV v2 data and metadata activity change?
- Did the aggregate KV secret count change over the usage-gauge window?
- Did database credential, role, config, or root-rotation paths appear?
- Did Transit cryptographic operation paths appear?
- Did PKI role, issue, issuer, root, certificate, tidy, or revoke paths appear?
- Which secret engine responses include errors or permission denials?

## What this dashboard is not for

Do not use this dashboard as a mount inventory source of truth. It reads audit
events that Loki collected. Use OpenBao APIs or configuration state when you
need the current mount table.

Do not use this dashboard as a replacement for secret-engine-specific
dashboards or health checks. Some engines need deeper checks, such as PKI
issuer health, database connection checks, or Transit key policy review.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | UID | Used for |
| ----------- | --- | -------- |
| Prometheus | `prometheus` | Mount inventory, KV secret count, KV route, dynamic lease creation, PKI operation, and database operation metrics. |
| Loki | `loki` | Secret engine and mount audit streams. |

The log panels query audit logs collected with `log_stream="openbao.audit"` and
a bounded `node_id` label.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Request ID | Textbox | `.*` | Narrow the stream to one request ID or pattern. |
| Engine or mount path | Textbox | `.*` | Narrow the stream to a mount or engine path pattern. |
| Operation | Custom | `.*` | Filter to `read`, `list`, `create`, `update`, or `delete`. |
| Node | Textbox | `.*` | Narrow the stream to one OpenBao node label. |

Treat textbox values as LogQL regular expressions. Escape special characters
when you need an exact match.

## How to read mount lifecycle activity

Mount lifecycle panels filter `sys/mounts` paths. These events can represent
mount table reads and mount lifecycle changes such as enable, tune, or disable
activity.

Mount inventory panels use `core_mount_table_num_entries` and
`core_mount_table_size` recording rules. These metrics group by bounded mount
metadata such as type and local status, not by mount path. Use them to detect
growth or configuration churn before moving to restricted audit investigation.

Confirm that mount lifecycle changes match approved change windows. Mount path
changes can alter routing, policy boundaries, secret engine behavior, and
application access.

## How to read KV activity

KV panels filter `secret/data/.*`, `secret/metadata/.*`, and `kv-v1/.*` paths,
which match the reference stack's KV v2 and KV v1 mount paths.

KV secret count panels use the `secret_kv_count` recording rules. OpenBao
emits this as a slow usage gauge, so the dashboard reads the maximum observed
value over 30 minutes. Use it for trends and broad namespace-level inventory,
not as exact request-time state.

KV v1 and KV v2 operation panels use route-derived metrics for the reference
`kv-v1/` mount and the default `secret/` KV v2 mount. These panels show create,
read, list, and delete rates and average latency when OpenBao emits those route
summary metrics. Custom KV mount paths produce different route metric names.

KV path names can reveal business context. Keep request paths out of Loki
labels and restrict dashboard access.

## How to read database activity

Database panels filter configuration, role, credential, and root-rotation
paths. These events can point to dynamic credential generation, role changes,
connection configuration changes, or root credential rotation.

Use the [OpenBao database secrets dashboard](./database-secrets.md) for focused
database operation latency, failure, lease, and audit investigation. Use the
database engine's own operational checks when you need to prove connection
health or credential revocation behavior.

## How to read Transit activity

Transit panels filter key management and cryptographic operation paths such as
encrypt, decrypt, rewrap, sign, verify, and HMAC.

Transit activity can represent application cryptographic workflows. Treat
unexpected spikes as workload or security signals and correlate them with
application changes.

Use the [OpenBao Transit dashboard](./transit.md) for focused key management,
cryptographic operation, denied request, and response error investigation.

## How to read PKI activity

PKI panels filter roles, issue, issuer, root, certificate, tidy, and revoke
paths. Use them to inspect audited certificate lifecycle activity when the
deployment enables PKI.

PKI audit activity does not replace PKI-specific health checks, issuer checks,
CRL behavior, or certificate expiry monitoring.

Use the [OpenBao PKI dashboard](./pki.md) for focused issue and revoke metrics,
certificate lifecycle audit streams, issuer activity, and response errors.

## How to read errors and denied requests

Response error panels show audited secret engine response entries that include
error fields. Denied request panels filter error text that includes
`permission denied`.

Use these panels to identify policy, path, or backend problems. Then inspect
the filtered stream and the relevant OpenBao policy or secret-engine
configuration.

## Label safety

The dashboard uses query-time parsing for request IDs, request paths, and
operations. It does not require engine paths or secret paths as Loki labels.

Keep mount paths, secret paths, request IDs, token accessors, entity IDs, and
client addresses out of labels unless your organization approves the
cardinality and metadata exposure tradeoff.

## Common mistakes

- Treating audit event volume as current mount inventory.
- Promoting mount paths or secret paths to Loki labels.
- Copying path-heavy dashboard patterns without validating mount names.
- Treating permission denied counts as malicious before checking policy
  changes and expected denied tests.
- Expecting one dashboard to provide full PKI, database, Transit, and KV health.

## Known limitations

- The dashboard assumes default reference paths such as `secret`, `kv-v1`,
  `database`, `transit`, and `pki`.
- Custom mount paths need filter updates or matching regular expressions.
- The dashboard depends on Loki retention for `log_stream="openbao.audit"`.
- It cannot show paths that bypass the audit system.
- It does not replace secret-engine-specific health checks.
- KV secret count depends on the `usage_gauge_period` collector and may lag
  recent writes or deletes.
- KV route metrics are mount-path-derived; the generated KV route panels
  cover the reference `kv-v1/` and default `secret/` mounts, not every
  possible KV mount name.

## What's next

- Use [OpenBao audit investigation dashboard](./audit-investigation.md) for
  broader request ID, path, operation, and node drilldown.
- Use [OpenBao secret engine metrics](../metrics/secret-engine-metrics.md) for
  metric names, recording rules, and gauge behavior.
- Use [OpenBao database secrets dashboard](./database-secrets.md) for focused
  database secrets engine investigation.
- Use [OpenBao Transit dashboard](./transit.md) for focused Transit key
  management and cryptographic operation investigation.
- Use [OpenBao PKI dashboard](./pki.md) for focused PKI operation metrics and
  certificate lifecycle investigation.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add mount, engine, or path labels.
- Use [Metrics, logs, and audit logs](../concepts/metrics-vs-logs-vs-audit-logs.md)
  to decide whether the question belongs in audit logs or metrics.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  review audit collection and access boundaries.

Source: OpenBao documents secrets engines and mount-path behavior in the
[OpenBao secrets engines documentation][openbao-secrets]. OpenBao documents KV,
database, Transit, and PKI behavior in the
[OpenBao KV documentation][openbao-kv],
[OpenBao database secrets engine documentation][openbao-database],
[OpenBao Transit documentation][openbao-transit], and
[OpenBao PKI documentation][openbao-pki]. This page describes the generated
dashboard contract in
`contracts/dashboards/openbao-secret-engines-mounts.yaml`.

[openbao-database]: https://openbao.org/docs/secrets/databases/
[openbao-kv]: https://openbao.org/docs/secrets/kv/
[openbao-pki]: https://openbao.org/docs/secrets/pki/
[openbao-secrets]: https://openbao.org/docs/secrets/
[openbao-transit]: https://openbao.org/docs/secrets/transit/
