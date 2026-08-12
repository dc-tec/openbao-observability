# OpenBao database secrets dashboard

Use this explainer to read the generated OpenBao database secrets dashboard. It
is for operators who need focused visibility into database secrets engine
credential activity, database operation latency, database operation failures,
and related audit events.

## What this dashboard is for

Use the database secrets dashboard when you need to inspect dynamic database
credential behavior after the overview, token and lease lifecycle, or secret
engines dashboard points to database-specific pressure.

The dashboard answers these questions:

- Are database credential create, renew, or revoke operations active?
- Are database operation failures increasing?
- Is database operation latency elevated?
- Are database dynamic leases being created?
- Did database connection, role, static role, root-rotation, or credential
  paths appear in audit logs?
- Did database lease lookup, renew, or revoke requests occur?
- Which database audit responses include errors?

## What this dashboard is not for

Do not use this dashboard as a database health check for PostgreSQL, MySQL,
MariaDB, Cassandra, InfluxDB, Valkey, or a custom database plugin. It shows
OpenBao-side database secrets engine behavior.

Do not use this dashboard as the source of truth for current database users,
database grants, static role state, or root credential state. Use your database
platform and OpenBao configuration APIs for that.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | UID | Used for |
| ----------- | --- | -------- |
| Prometheus | `prometheus` | Database operation rates, latency, failure counters, and lease creation metrics. |
| Loki | `loki` | Database secrets engine audit request and response streams. |

The metric panels use normalized `openbao:` recording rules. The log panels
parse audit JSON at query time and require `log_stream="openbao.audit"`.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Cluster | Textbox | `.*` | Selects the stable OpenBao cluster identity. |
| Kubernetes namespace | Textbox | `.*` | Selects the OpenBao workload namespace when multiple OpenBao instances share the same observability backend. |
| Scrape profile | Textbox | `.*` | Selects the active or all-node metrics profile. |
| Request ID | Textbox | `.*` | Narrow the stream to one request ID or pattern. |
| Database mount path | Textbox | `database` | Select the database secrets engine mount path. |
| OpenBao namespace | Textbox | `.*` | Filters the logical OpenBao namespace. |
| Operation | Custom | `.*` | Filter to `read`, `list`, `create`, `update`, or `delete`. |
| Node | Textbox | `.*` | Narrow the stream to one OpenBao node label. |

Treat textbox values as regular expressions. Use the mount path without a
trailing slash, such as `database` or `prod-database`.

## How to read operation metrics

Use operation-rate panels to understand whether OpenBao is creating, renewing,
or revoking database credentials. These panels use generic database middleware
metrics rather than plugin-specific PostgreSQL, MySQL, or other plugin metric
families.

Use latency panels to compare create, renew, revoke, initialize, and close
latency. Elevated latency can come from OpenBao storage pressure, Raft pressure,
database connection limits, database locks, slow credential-management
statements, or plugin behavior.

OpenBao database telemetry describes create, renew, and revoke user operations.
The OpenBao `2.6.0` Prometheus fixture emits the generic operation families as
`database_NewUser`, `database_UpdateUser`, and `database_DeleteUser`. The
generated recording rules expose those as create, renew, and revoke dashboard
signals.

The database dynamic lease panels use `secret_lease_creation` recording rules.
The namespace drilldown keeps only the bounded `namespace` and `secret_engine`
dimensions. Use it to confirm whether dynamic database leases are concentrated
in root, a tenant namespace, or an unexpected namespace before you inspect
specific audit events.

## How to read failure panels

Failure panels show database initialize, create, renew, revoke, and close
errors when OpenBao emits the corresponding counters. Treat failures as a
signal that either the database secrets engine configuration or the external
database dependency needs investigation.

Create failures usually point to role statements, plugin configuration,
database permissions, connection limits, or database availability. Renew and
revoke failures can leave credentials valid longer than intended. Use the
secret engine feature warning runbook when an alert fires.

## How to read audit panels

Database audit panels separate four paths:

| Path group | What it shows |
| ---------- | ------------- |
| Config and role stream | Connection config, dynamic roles, static roles, root rotation, and static role rotation. |
| Credential stream | Dynamic credential reads under `creds` and static credential reads under `static-creds`. |
| Lease stream | Lease lookup, renew, and revoke requests for database dynamic credentials. |
| Response error stream | Database secrets engine responses with audit error fields. |

Audit logs can contain sensitive metadata even when OpenBao HMACs field values.
Keep this dashboard in a restricted Grafana folder.

## Label safety

The dashboard parses request IDs, request paths, operations, and audit errors at
query time. It uses the OpenBao namespace metric label only for bounded
database lease drilldown. It does not require database role names, credential
paths, lease IDs, request IDs, client addresses, token accessors, or entity IDs
as Prometheus or Loki labels.

Keep this pattern when you extend the dashboard. Database role names and mount
paths can reveal application architecture.

## Common mistakes

- Treating credential read volume as the number of active database users.
- Treating missing database metrics as proof that the database engine is idle.
- Grouping shared dashboards by database role, username, lease ID, or client
  identity.
- Investigating OpenBao latency without checking external database health.
- Rotating root credentials only to clear a warning.
- Forgetting that static role credential reads and dynamic role credential
  reads have different operational meaning.

## Known limitations

- The dashboard assumes the default database mount path is `database`.
- Custom mount paths need the `Database mount path` variable to match the
  deployment.
- Plugin-specific metric families are contract data, but the dashboard prefers
  generic database metrics.
- The current fixture validates root-namespace database behavior and database
  lease lookup, renew, and revoke behavior inside the `team-a` namespace. Other
  namespace layouts still need local validation before you depend on namespace
  grouping.
- The dashboard depends on Loki retention for `log_stream="openbao.audit"`.

## What's next

- Use [Secret engine feature warnings](../runbooks/secret-engine-feature-warnings.md)
  when database warnings fire.
- Use [OpenBao token and lease lifecycle dashboard](./token-lease-lifecycle.md)
  when database lease pressure appears.
- Use [OpenBao secret engines and mounts dashboard](./secret-engines-mounts.md)
  when you need a broader secret-engine view.
- Use [OpenBao runtime and storage dashboard](./runtime-storage.md) when
  database latency correlates with storage or runtime pressure.

Source: OpenBao documents database secrets engine behavior in the
[OpenBao database secrets engine documentation][openbao-database]. OpenBao
documents database telemetry in the
[OpenBao database telemetry documentation][openbao-database-metrics]. This page
describes the generated dashboard contract in
`contracts/dashboards/openbao-database-secrets.yaml`.

[openbao-database]: https://openbao.org/docs/secrets/databases/
[openbao-database-metrics]: https://openbao.org/docs/internals/telemetry/metrics/database/
