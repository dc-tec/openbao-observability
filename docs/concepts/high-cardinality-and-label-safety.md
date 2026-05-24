# High-cardinality and label safety

Use this explainer before you add OpenBao fields to Prometheus labels, Loki
labels, dashboard variables, or alert groupings. It is for operators who need
to protect performance and avoid exposing sensitive operational metadata.

## Why this matters

Labels define how Prometheus and Loki index data. Every extra label dimension
can multiply the number of metric series or log streams. Unbounded labels can
make queries expensive, create noisy dashboards, and expose values that belong
in restricted investigation views.

OpenBao also has security-specific label risks. Request paths, mount paths,
entity identifiers, token accessors, auth accessors, and client addresses can
reveal how your secrets platform is used.

## Mental model

Cardinality is the number of unique label combinations in a data set.

Prometheus stores a separate time series for each unique metric name and label
set. Loki stores log entries in streams identified by labels. High-cardinality
values create many series or streams, and unbounded values create growth you
cannot plan around.

Use labels for stable source identity and bounded routing dimensions. Keep
investigation fields in the log body, structured metadata, or query-time JSON
parsing.

## Unsafe label candidates

| Candidate | Risk | Safer pattern |
| --------- | ---- | ------------- |
| `request_id` | Unbounded value, high investigation value, and poor grouping dimension. | Parse at query time in restricted dashboards. |
| `request_path` | Can expose secret paths, system usage, and mount layout. | Filter with LogQL JSON parsing or bounded dashboard variables. |
| `secret_path` | Can expose secret naming and business context. | Keep in audit logs and restrict dashboard access. |
| `mount_path` | Can contain slashes, tenant names, application names, or dynamic paths. | Group by secret engine type when available. |
| `namespace_path` | Can expose tenancy and organizational structure. | Use only when the namespace set is bounded and approved. |
| `entity_id` | Security-sensitive identity identifier. | Parse at query time for investigations. |
| `token_accessor` | Security-sensitive token metadata. | Keep out of labels and restrict audit access. |
| `auth_accessor` | Security-sensitive auth mount metadata. | Use bounded auth method type when available. |
| `client_ip` | High-cardinality and privacy-sensitive value. | Use network telemetry or restricted audit queries. |
| `policy` | Can expose authorization model details and grow over time. | Use only after cardinality review and access review. |

## Prometheus guidance

Keep metric labels bounded and operationally useful. Prometheus warns that
every unique label combination creates a new time series and that unbounded
labels can dramatically increase stored data.

For OpenBao metrics:

- Prefer normalized recording rules for dashboards and alerts.
- Validate the live label set before grouping by `cluster`, `namespace`,
  `mount_point`, `policy`, `instance`, or `pod`.
- Avoid grouping overview panels by request path, secret path, token accessor,
  entity ID, or client address.
- Treat `mount_point`, `namespace`, and `policy` as advanced dimensions because
  they can encode organizational structure.
- Keep alert labels concise so alert routing does not leak sensitive values.

Some OpenBao metrics include labels that are useful but sensitive. The metric
contract documents which labels this project uses in generated queries.

## Loki guidance

Keep Loki labels small, stable, and source-oriented. Loki documentation
recommends low-cardinality labels and structured metadata for frequently
searched high-cardinality values.

For OpenBao logs:

- Keep `log_stream="openbao.operational"` for operational logs.
- Keep `log_stream="openbao.audit"` for audit logs.
- Use node, job, environment, and tenant labels only when they are bounded and
  approved.
- Parse `request.path`, `request.id`, `auth.entity_id`, `auth.client_token`,
  and related audit fields at query time.
- Restrict dashboards that expose audit fields, even when those fields are not
  labels.

Do not promote audit fields to Loki labels because they are convenient for one
dashboard. A label choice changes ingestion, storage, query cost, and exposure
for every user of the Loki tenant.

## Dashboard guidance

Use dashboard variables for bounded dimensions that operators repeatedly filter
by. Avoid variables for unbounded values such as request IDs or secret paths.

For investigation workflows, prefer a search input or query-time filter that
does not alter ingestion labels. The audit investigation dashboard follows this
pattern for request ID drilldown.

## Design recommendations

Use a label only when all of these conditions hold:

- The value set is bounded and small enough for your retention window.
- The value does not reveal sensitive OpenBao usage patterns.
- Operators query by the label often enough to warrant indexing it.
- Alert routing benefits from the label.
- The value does not change shape when teams add mounts, policies, or auth
  methods.

When a value fails any condition, keep it out of labels. Parse it at query time
or store it in a restricted backend designed for investigation.

## Common mistakes

- Labeling every JSON field because the collector can extract it.
- Using request paths as labels for audit logs.
- Grouping alert instances by token accessor, entity ID, or client address.
- Copying path-heavy dashboard panels without validating mount names and label
  cardinality.
- Treating a low-cardinality demo fixture as proof that a production label is
  safe.
- Ignoring that labels can leak metadata even when secret values are HMACed.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed upstream behavior | Prometheus and Loki documentation warn against high-cardinality or unbounded labels. OpenBao documents audit fields, HMAC behavior, and metric labels such as `mount_point`, `namespace`, and `policy`. |
| Observed fixture behavior | The Docker Compose fixture uses low-cardinality `log_stream` labels and query-time parsing for audit fields. |
| Design decision | This project treats request paths, IDs, entity IDs, token accessors, auth accessors, and client addresses as investigation fields instead of labels. |
| To validate | Your production cardinality, tenant boundaries, retention windows, and access-control requirements. |

## What's next

- Use [OpenBao observability model](./openbao-observability-model.md) to see
  where labels fit in the signal pipeline.
- Use [Metrics, logs, and audit logs](./metrics-vs-logs-vs-audit-logs.md) to
  choose the right signal before you add labels.
- Use [Understand metric prefixes and recording rules](../contracts/metric-prefix.md)
  before you group raw OpenBao metrics.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) for
  the audit-log collection pattern used by this project.

Source: Prometheus documents label cardinality in the
[Prometheus metric and label naming documentation][prometheus-labels]. Loki
documents label cardinality and structured metadata in the
[Grafana Loki label documentation][loki-labels] and
[Grafana Loki label best practices][loki-label-best-practices]. OpenBao
documents audit fields and HMAC behavior in the
[OpenBao audit device documentation][openbao-audit], and metric labels in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics].

[loki-label-best-practices]: https://grafana.com/docs/loki/latest/get-started/labels/bp-labels/
[loki-labels]: https://grafana.com/docs/loki/latest/get-started/labels/
[openbao-audit]: https://openbao.org/docs/audit/
[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
[prometheus-labels]: https://prometheus.io/docs/practices/naming/
