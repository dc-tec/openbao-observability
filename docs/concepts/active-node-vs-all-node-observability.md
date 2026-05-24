# Active-node and all-node observability

Use this explainer to choose between active-node and all-node OpenBao metrics
scraping. It is for operators who need to understand what each scrape profile
shows, what it hides, and what security controls it requires.

## Why this matters

OpenBao HA clusters have one active node and one or more standby or follower
nodes. A dashboard can only show the metrics that Prometheus collects. If you
scrape only the active node, you get a simpler and safer cluster-level view,
but you lose detail about standby and follower runtime state.

All-node scraping adds that detail. It also expands the metrics exposure
surface, so it needs stricter network and access controls.

## Mental model

Use the scrape profile to match the operational question.

| Question | Best profile |
| -------- | ------------ |
| Is the OpenBao service reachable? | Active-node scrape. |
| Does the cluster have one active node? | Active-node or all-node scrape. |
| Are all nodes unsealed? | All-node scrape. |
| Is a standby node unhealthy? | All-node scrape. |
| Are Raft followers behind? | All-node scrape. |
| Are Raft non-voters or read replicas healthy? | All-node scrape. |
| Is request latency rising on the active node? | Active-node scrape. |
| Does every node expose expected runtime metrics? | All-node scrape. |

Active-node scraping is a cluster lens. All-node scraping is a node lens.

## OpenBao behavior

OpenBao documents the Prometheus `/v1/sys/metrics` endpoint as accessible only
on active nodes by default. Standby-node access requires unauthenticated
metrics access on the listener that exposes metrics.

That behavior makes secure active-node scraping the default production
baseline. It also means standby visibility needs a deliberately isolated
metrics path, not a casual change to the main client listener.

## Active-node scraping

Active-node scraping targets the active OpenBao node, usually through an active
service or equivalent service-discovery selector.

Use it when you need:

- A secure default for production.
- Cluster-level health.
- Active request latency and throughput.
- Active-node audit metrics.
- Fewer scrape targets and simpler access control.

Active-node scraping hides or weakens:

- Standby runtime pressure.
- Standby scrape failures.
- Follower Raft detail.
- Per-node unseal visibility.
- Node-local operational differences.

## All-node scraping

All-node scraping targets every OpenBao node. In Kubernetes, that usually means
a service or pod monitor that selects all OpenBao pods, not only the active
pod.

Use it when you need:

- HA and Raft diagnostics.
- Standby and follower visibility.
- Read-replica or non-voter visibility.
- Per-node unseal status.
- Node-local runtime trends.
- Autopilot and peer-health dashboards with stronger context.

All-node scraping requires stronger controls because standby metrics access
depends on unauthenticated metrics access. Use a metrics-only listener, network
policy, firewall rules, and a dedicated scrape path that general clients cannot
reach.

## Security model

The metrics endpoint can reveal operational metadata such as cluster labels,
runtime pressure, auth method labels, namespace labels, mount points, policies,
and Raft peer identifiers. Treat metrics access as operationally sensitive.

For the active-node profile:

- Require an OpenBao token for `/v1/sys/metrics`.
- Use TLS.
- Scope the token to metrics access only.
- Rotate the token through your normal secret-management process.

For the all-node profile:

- Use a private metrics-only listener.
- Allow only Prometheus or the approved metrics collector.
- Keep the listener off public and application networks.
- Review labels before you expose the metrics to shared tenants.
- Monitor the scrape targets themselves.

## Dashboard impact

The same dashboard can mean different things under each profile.

| Dashboard area | Active-node scrape | All-node scrape |
| -------------- | ------------------ | --------------- |
| Overview health | Good cluster-level view. | Better node-level context. |
| Request health | Strong active-node view. | Adds node context where metrics exist. |
| HA/Raft | Limited follower detail. | Stronger peer and follower detail. |
| Runtime pressure | Active-node pressure only. | Per-node pressure. |
| Alerts | Good for cluster-critical alerts. | Better for node-specific alerts. |

Document the scrape profile beside your dashboards. Empty or missing panels
can mean a dashboard assumption mismatch rather than an OpenBao incident.

## Namespace and scale impact

Namespaces and read-replica topologies make scrape-profile documentation more
important. A namespace label can be useful for tenant-level analysis, but it can
also expose organizational structure and increase cardinality. A non-voter can
be a healthy unsealed OpenBao node without increasing Raft quorum failure
tolerance.

Use all-node scraping for read-replica and non-voter diagnostics. Keep quorum
alerts based on voter-aware signals such as Autopilot health, failure
tolerance, and peer state. Do not infer voter count from the number of scraped
or unsealed nodes.

## Design recommendations

Use authenticated active-node scraping as the default production baseline.

Add private all-node scraping when you operate HA/Raft clusters and need
standby, follower, or node-level diagnostics. Treat it as an elevated
observability profile with a narrower network path and explicit approval.

Keep dashboard and alert contracts honest about which profile they require.
Avoid writing all-node assumptions into panels that users expect to work with
the secure active-node baseline.

## Common mistakes

- Expecting active-node scraping to show standby runtime pressure.
- Enabling unauthenticated metrics access on a general client listener.
- Treating an empty follower panel as proof that Raft is broken.
- Forgetting to isolate the all-node metrics listener.
- Grouping all-node metrics by sensitive or high-cardinality labels.
- Changing scrape labels without updating dashboard and alert assumptions.

## Evidence basis

| Classification | Meaning in this project |
| -------------- | ----------------------- |
| Confirmed OpenBao docs behavior | OpenBao documents active-node-only Prometheus metrics access by default and standby access through unauthenticated metrics access. |
| Observed fixture behavior | The local OpenBao 2.5.4 HA fixture uses all-node scraping to validate per-node, three-voter Raft, and `team-a` namespace behavior. |
| Design decision | This project treats active-node scraping as the secure baseline and all-node scraping as an elevated HA/Raft diagnostics profile. |
| To validate | Kubernetes service labels, scrape identities, listener isolation, feature-specific namespace label policy, non-voter metrics, and read-replica behavior in your production environment. |

## What's next

- Use [Configure a secure metrics scrape](../metrics/secure-metrics-scrape.md)
  for the authenticated active-node baseline.
- Use [Configure an all-node metrics scrape](../metrics/all-node-metrics-scrape.md)
  for the private all-node profile.
- Use [OpenBao HA/Raft observability](./openbao-ha-raft-observability.md) to
  understand why HA/Raft troubleshooting benefits from all-node visibility.
- Use [Namespaces and scale observability](./namespaces-and-scale-observability.md)
  before you add namespace or read-replica panels.
- Use [High-cardinality and label safety](./high-cardinality-and-label-safety.md)
  before you expose or group additional metric labels.
- Use [OpenBao overview dashboard](../dashboards/overview-dashboard.md) to
  understand how scrape profile changes dashboard interpretation.

Source: OpenBao documents Prometheus telemetry behavior and standby metrics
access in the [OpenBao telemetry documentation][openbao-telemetry]. OpenBao
documents the metrics endpoint in the
[OpenBao metrics API documentation][openbao-metrics-api].

[openbao-metrics-api]: https://openbao.org/api-docs/system/metrics/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
