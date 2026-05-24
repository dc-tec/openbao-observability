# OpenBao namespaces and scale dashboard

Use this explainer to read the generated OpenBao namespaces and scale
dashboard. It is for operators who need to inspect namespace-aware activity,
Raft voters, non-voter read replicas, and all-node scrape signals without
turning namespace paths or request paths into labels.

## What this dashboard is for

Use this dashboard when you need topology context that sits between the
overview dashboard and the deeper HA/Raft, token, lease, and secret-engine
dashboards.

The dashboard answers these questions:

- Which namespace IDs have audited activity or response errors?
- Are token creation and dynamic lease signals concentrated in one namespace?
- Is mount table size or mount count changing?
- How many Raft peers are visible?
- Does Autopilot report enough voter failure tolerance?
- Are any Raft nodes unhealthy?
- Are voters or read replicas showing apply lag or pending FSM work?

## What this dashboard is not for

Do not use this dashboard as a namespace inventory source of truth. It uses
metrics and audit events that the observability stack collected. Use OpenBao
namespace APIs or approved configuration state when you need authoritative
namespace inventory.

Do not use the number of unsealed or scraped nodes as voter count. Unsealed
nodes can include voters, standbys, non-voters, read replicas, or nodes that
are still catching up.

Do not treat non-voter or read-replica symptoms as quorum loss by default.
Read-replica failure can reduce read capacity or diagnostic coverage without
changing voter failure tolerance.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | UID | Used for |
| ----------- | --- | -------- |
| Prometheus | `prometheus` | Namespace-aware token and lease rules, mount table rules, Raft and Autopilot rules. |
| Loki | `loki` | Audit and operational log streams. |

The Raft and read-replica panels require the all-node metrics profile for the
strongest signal coverage. Active-node scraping can still show cluster-level
signals, but it usually weakens node and follower diagnostics.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| OpenBao namespace | Textbox | `.*` | Filters namespace-aware metrics and audit `request.namespace.id`. |
| Node | Textbox | `.*` | Filters OpenBao node labels in logs and node-aware metrics. |
| Raft peer | Textbox | `.*` | Filters Raft storage statistics by peer ID. |
| Request ID | Textbox | `.*` | Narrows audit streams to one request ID or pattern. |
| Operation | Custom | `.*` | Filters audited request operations. |

Treat textbox values as regular expressions. For audit logs, the namespace
filter matches the audit namespace ID, such as `root` in the root namespace.
It does not expose namespace paths as Loki labels.

## How to read namespace activity

The top row combines audit volume with namespace-aware token and lease
metrics. Audit panels parse `request.namespace.id` at query time. Metrics
panels use normalized recording rules that keep namespace as a bounded
Prometheus dimension.

Use these panels to spot whether activity has shifted into an unexpected
namespace before you drill into request IDs, auth paths, secret-engine paths,
or policies.

Namespace response errors need context. A failed request can be an expected
denied access test, a policy problem, a missing mount, or a backend issue.
Use the error stream before treating the count as malicious or service-impacting.

## How to read namespace token and lease pressure

Token creation by namespace and auth method helps identify which namespace and
auth source is creating session pressure. Dynamic lease creation by namespace
and engine helps identify where database or other dynamic secret usage is
concentrated.

Do not use these panels for user attribution. They intentionally avoid token
accessors, entity IDs, client addresses, mount paths, and policy names.

## How to read mount table signals

Mount table panels show count and size by bounded `type` and `local` labels.
They are coarse topology signals. A rising mount table can reflect planned
tenant onboarding, namespace expansion, auth method rollout, or secret engine
enablement.

Use OpenBao APIs or configuration state when you need exact mount paths,
current mount options, or namespace inventory.

## How to read scale and read-replica signals

Raft peer count shows how many peers the normalized rules can observe. In a
topology with three voters and one non-voter read replica, a peer count of
four is expected, but failure tolerance still comes from voters.

Failure tolerance is the quorum-oriented signal. Healthy node count and
unhealthy node count are node health signals. Do not add non-voters to the
failure tolerance calculation.

Raft apply gap compares commit index and applied index by peer. A sustained
gap can indicate a follower or read replica falling behind. FSM pending work
can show apply pressure. Interpret both with operational logs, restarts, and
known maintenance windows.

## Label safety

This dashboard uses query-time parsing for request IDs, namespace IDs,
operations, and request paths. It does not require namespace paths, request
paths, mount paths, entity IDs, token accessors, auth accessors, usernames, or
client addresses as labels.

Keep the dashboard restricted when namespace names or IDs reveal tenant or
organizational structure.

## Common mistakes

- Treating namespace audit activity as namespace inventory.
- Treating read-replica failure as voter quorum loss.
- Treating unsealed node count as voter count.
- Promoting namespace paths or request paths into Loki labels.
- Grouping shared dashboards by token accessor, entity ID, mount path, or
  policy.
- Assuming root namespace PKI metrics automatically cover non-root namespace
  PKI metrics.

## Known limitations

- Namespace-heavy panels depend on the namespace labels and audit namespace
  IDs collected by your deployment.
- Broader nested namespace behavior is still an extension beyond the current
  fixture scope.
- Operator-managed read-replica behavior should be validated in the operator
  integration profile before you depend on production thresholds.
- Active-node scraping may leave follower, standby, and read-replica panels
  empty or incomplete.
- Namespace paths are deliberately not labels, so path-level namespace
  investigation remains a restricted audit-log workflow.

## What's next

- Use [Namespaces and scale observability](../concepts/namespaces-and-scale-observability.md)
  to understand the topology model behind this dashboard.
- Use [Active-node and all-node observability](../concepts/active-node-vs-all-node-observability.md)
  to choose the right scrape profile.
- Use [OpenBao HA/Raft dashboard](./ha-raft.md) for deeper Raft and Autopilot
  troubleshooting.
- Use [OpenBao token and lease lifecycle dashboard](./token-lease-lifecycle.md)
  when namespace token or lease pressure needs a lifecycle drilldown.
- Use [OpenBao secret engines and mounts dashboard](./secret-engines-mounts.md)
  when namespace pressure points to engine or mount activity.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add namespace, mount, request, or identity dimensions.

Source: OpenBao namespace and Raft scale behavior are discussed in
[Namespaces and scale observability](../concepts/namespaces-and-scale-observability.md).
This page describes the generated dashboard contract in
`contracts/dashboards/openbao-namespaces-scale.yaml`.
