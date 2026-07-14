# Namespaces and scale observability

Use this explainer to understand how namespaces, Raft voters, and Raft
non-voters affect OpenBao observability. It is for operators who need to plan
dashboard, alert, and fixture coverage beyond the baseline HA cluster.

## Why this matters

OpenBao deployments can grow along several axes. You can add namespaces to
separate tenants or administrative domains, add Raft voters to change quorum
size and failure tolerance, or add Raft non-voters to receive the replication
stream without participating in quorum.

Those changes affect how you read metrics and logs. They also affect which
labels are safe to expose, which dashboards need all-node scraping, and which
claims this reference architecture can validate with the local fixture.

## Mental model

Treat namespace and scale signals as topology context, not as default
dashboard groupings.

| Dimension | What changes | Observability impact |
| --------- | ------------ | -------------------- |
| Namespace count | More logical OpenBao partitions. | More labels and audit context to protect. |
| Mount count | More auth and secrets engines per namespace. | Larger mount tables and more path-derived signals. |
| Raft voters | More nodes in quorum decisions. | Different peer count and failure tolerance. |
| Raft non-voters | More replicated nodes outside quorum. | More all-node targets and more peer states to classify. |
| Read traffic | More traffic can land away from the leader when read-scaling features are used. | Request and latency signals need node role context. |

Do not treat the number of scraped unsealed nodes as the number of Raft voters.
Unsealed nodes can include active nodes, standby nodes, voters, non-voters, or
nodes that are still catching up.

## Namespace signals

OpenBao exposes namespace context in several places:

- Some token, lease, identity, and secret-engine metrics can include a
  `namespace` label.
- Audit entries can include request namespace context.
- The `bao namespace` command and `sys/namespaces` API expose namespace
  inventory.
- Mount table size and mount count signals grow as namespaces add auth and
  secrets engine mounts.

Use namespace data when you need tenant or administrative-domain context. Keep
namespace labels out of broad overview panels unless the namespace set is
bounded and approved for the metrics or logging tenant.

Some feature metrics also change metric-family names when they run inside a
namespace. The namespace PKI fixture emits `<prefix>_<sanitized_namespace>_pki_issue`
style families instead of only adding a `namespace` label to the root PKI
metric family. Do not hardcode fixture namespace names in dashboards.

## Scale signals

For voter-based HA, use the HA/Raft dashboard to read active node count,
unsealed node count, Raft peer count, Autopilot health, failure tolerance, and
follower progress.

For read-replica or non-voter topologies, add role awareness before you alert.
OpenBao documents non-voters as nodes that receive the Raft data replication
stream without participating in quorum. Autopilot output can show `leader`,
`voter`, and `non-voter` states.

This distinction matters because a non-voter can be healthy and unsealed
without increasing quorum failure tolerance. A non-voter failure can reduce
read capacity or diagnostic coverage without creating the same quorum risk as a
voter failure.

## Current validation scope

This project currently validates a focused local topology:

| Area | Current status |
| ---- | -------------- |
| OpenBao version | Captured with OpenBao 2.6.0 fixtures. |
| Namespace scope | Root namespace, one child namespace named `team-a`, and a minimal nested namespace path named `team-a/payments`. |
| Namespace activity | Userpass, AppRole, token create/lookup/revoke, KV v2, transit, PKI issue/revoke/failure, database lease lookup/renew/revoke, policy writes, and audit request namespace fields. |
| Raft topology | Three voter nodes plus one non-voter read replica in the HA/Raft fixture. |
| Source prefixes | `vault` and `openbao` metric prefixes. |
| Read replicas | Basic read behavior, audit entries, Raft peer state, and Autopilot node health are fixture-tested. |
| Nested namespaces | Minimal KV activity is fixture-tested for `team-a/payments`; broader nested feature behavior is still an extension. |

Use the generated dashboards and alerts as validated for the current fixture
scope. Treat namespace-heavy panels, namespace alert routing, database lease
behavior outside the fixture shape, broader nested namespace behavior,
production read-replica capacity thresholds, and non-voter failure alerts as
extensions until you capture fixtures for those shapes.

## Design recommendations

Use these rules when you extend the reference architecture:

- Keep root namespace as the baseline fixture.
- Keep the non-root namespace fixture in place before you group dashboards by
  `namespace`.
- Add feature-specific namespace fixtures before you rely on namespace grouping
  for that feature.
- Keep read-replica alerts separate from voter quorum alerts.
- Keep `namespace`, `mount_point`, and `policy` out of default alert labels.
- Use all-node scraping for HA/Raft and read-replica diagnostics.
- Separate voter quorum alerts from read-capacity or replica-coverage alerts.
- Document whether a panel counts nodes, voters, peers, or scrape targets.

These rules keep the default dashboards safe for shared operational use while
leaving a path for deeper production profiles.

## Common mistakes

- Treating every unsealed node as a Raft voter.
- Treating a non-voter failure as the same condition as voter quorum loss.
- Grouping overview panels by namespace before reviewing cardinality and access
  control.
- Treating the `team-a` and `team-a/payments` fixtures as proof of all nested
  namespace or feature-specific behavior.
- Adding replication alerts without separating quorum health, read capacity,
  and scrape target health.
- Using namespace names or paths as Loki labels without an access review.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents namespace commands, namespace limits, namespace-related lease metrics, Raft non-voters, and Autopilot states. |
| Observed fixture behavior | The local OpenBao 2.6.0 fixtures validate root namespace, the `team-a` child namespace across several auth and secrets-engine paths, minimal `team-a/payments` nested KV behavior, namespaced database leases, and HA/Raft behavior with three voters plus one non-voter read replica. |
| Design decision | This project treats namespace and read-replica coverage as explicit topology extensions, not default assumptions. |
| To validate | Broader nested namespace feature behavior, operator-managed read-replica behavior, and production alert thresholds. |

## What's next

- Use [Active-node and all-node observability](./active-node-vs-all-node-observability.md)
  to choose the scrape profile for HA and read-replica diagnostics.
- Use [OpenBao HA/Raft observability](./openbao-ha-raft-observability.md) to
  understand quorum, failure tolerance, and follower progress.
- Use [OpenBao HA/Raft metrics](../metrics/ha-raft-metrics.md) to connect the
  topology model to Prometheus series.
- Use [OpenBao namespaces and scale dashboard](../dashboards/namespaces-scale.md)
  to inspect namespace-aware activity, non-voters, and read-replica signals.
- Use [High-cardinality and label safety](./high-cardinality-and-label-safety.md)
  before you expose namespace or mount dimensions.

Source: OpenBao documents namespace commands in the
[OpenBao namespace command documentation][openbao-namespace-command],
namespace limits in the [OpenBao limits documentation][openbao-limits],
namespace labels for lease metrics in the
[OpenBao telemetry configuration documentation][openbao-telemetry-config], and
Raft non-voters in the
[OpenBao Raft storage configuration documentation][openbao-raft-storage] and
[OpenBao Raft operator command documentation][openbao-raft-operator].

[openbao-limits]: https://openbao.org/docs/internals/limits/
[openbao-namespace-command]: https://openbao.org/docs/commands/namespace/
[openbao-raft-operator]: https://openbao.org/docs/commands/operator/raft/
[openbao-raft-storage]: https://openbao.org/docs/configuration/storage/raft/
[openbao-telemetry-config]: https://openbao.org/docs/configuration/telemetry/
