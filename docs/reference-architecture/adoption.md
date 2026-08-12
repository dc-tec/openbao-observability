# Adopt the reference architecture

Use this how-to to adapt the OpenBao Observability reference architecture to
your own monitoring, logging, dashboard, alerting, and security archive
platforms. It is for operators who want to preserve the architecture while
changing the implementation profile.

## Before you begin

- Read [Reference architecture overview](./overview.md).
- Identify the OpenBao version, topology, storage mode, and deployment platform.
- Identify the systems that own metrics, logs, audit archive, dashboards,
  alerting, paging, and runbooks in your environment.
- Confirm who can approve audit-log access, retention, and archive changes.

## Map source signals

1. Record the OpenBao metric prefix in use.

   OpenBao deployments commonly emit `vault_*` metrics unless you configure an
   `openbao` metrics prefix. The generated recording rules support both
   prefixes through contract generation.

2. Decide which OpenBao metrics your platform must collect.

   Start with the required metrics in
   [OpenBao core metric contract](../../contracts/metrics/openbao-core.yaml).
   Keep optional feature metrics separate from the first-stop overview path.

3. Separate operational logs from audit logs.

   Use the stream definitions in
   [OpenBao log stream contract](../../contracts/streams/log-streams.yaml).
   Keep `openbao.audit` and `openbao.audit_archive` in restricted paths.

4. Decide whether completed request logs are allowed.

   Keep completed request logging disabled by default. Use it as a temporary
   troubleshooting stream with short retention and restricted access.

5. Add platform signals that explain OpenBao failures.

   Include pod, host, volume, service discovery, network policy, and collector
   health signals that help explain why OpenBao cannot serve traffic, write
   audit logs, or expose metrics.

6. Declare required optional signals.

   Install an `openbao_observability_signal_expected` marker for each optional
   Kubernetes, probe, collector, or audit archive signal that your deployment
   requires. Use the [signal expectation example](../../examples/signal-expectations/).
   Do not install a marker for a signal that the deployment does not use.

## Choose collection profiles

1. Use authenticated active-node metrics scraping as the secure baseline.

   This profile gives you cluster-level health with a scoped metrics token. It
   does not give complete standby, sealed-node, or per-node Raft visibility.

2. Add private all-node scraping only when you need per-node visibility.

   The all-node profile needs a private metrics-only listener or equivalent
   local collection path. Restrict it with NetworkPolicy, firewall rules,
   private routing, mTLS proxying, or sidecar-local scraping.

3. Choose a log collector that preserves stream separation.

   The included profile uses Grafana Alloy. You can use another collector if it
   preserves the `openbao.operational`, `openbao.completed_requests`,
   `openbao.audit`, and `openbao.audit_archive` boundaries.

4. Keep audit archive delivery independent from dashboard exploration.

   Loki, OpenSearch, Elastic, Splunk, or another log backend can support
   investigation. Your compliance archive needs its own approved retention,
   integrity, and access model.

## Map labels and attributes

1. Start with the allowed and forbidden labels in the stream contract.

   Do not promote request paths, secret paths, request IDs, token accessors,
   entity identifiers, auth accessors, policies, or client addresses to labels.

2. Map platform labels to bounded dimensions.

   Common safe dimensions include environment, region, cluster, namespace, app,
   component, instance, pod, container, and deployment profile when those values
   are bounded and approved.

3. Keep investigation fields in the log body or backend-specific structured
   metadata.

   Parse sensitive audit fields at query time in restricted dashboards or
   investigation workflows.

## Port derived signals

1. Port metric recording rules before dashboards.

   Recording rules normalize raw OpenBao metrics and scrape labels into stable
   derived series. When your platform does not support PromQL, port the intent
   of each rule rather than copying the syntax.

2. Port alert contracts as named operational events.

   Preserve alert names, severity, signal type, summary, and runbook links when
   you move rules into another alerting system.

3. Port dashboard panels as questions.

   A dashboard panel asks an operational question, such as whether OpenBao has
   exactly one active node or whether audit request failures increased. Keep
   the question even when the query language or visualization changes.

4. Link runbooks from alerts and dashboards.

   Runbooks are part of the architecture. Keep them available from the system
   that pages your team.

## Verify the result

Your adopted profile is ready for review when these checks pass:

- Metrics collection shows the expected active-node or all-node targets.
- Recording rules or equivalent derived metrics evaluate for the expected
  OpenBao prefix.
- Operational logs and audit logs land in separate streams or indexes.
- Audit logs have restricted access and an archive path outside broad
  operational dashboards.
- Forbidden fields do not appear as metric labels, log labels, alert labels, or
  broad dashboard variables.
- Overview, HA/Raft, audit, operational log, token, lease, and runtime
  questions have dashboard coverage or documented exclusions.
- Alerts link to runbooks and include enough context for the on-call team to
  start the response.
- Fixture or staging validation exercises at least metrics scrape failure,
  sealed state, no active node, audit write failure, audit stream missing, and
  collector failure.

## What's next

- Use [Implementation profiles](../implementation-profiles/README.md) to choose
  the closest starting profile.
- Use [Prometheus, Loki, Grafana, and Alloy profile](../implementation-profiles/prometheus-loki-grafana-alloy.md)
  when you want to use the generated artifacts directly.
- Use [Secure metrics scrape](../metrics/secure-metrics-scrape.md) for the
  authenticated active-node baseline.
- Use [All-node metrics scrape](../metrics/all-node-metrics-scrape.md) when you
  need standby and per-node visibility.
- Use [Loki label strategy for OpenBao](../logging/loki-label-strategy.md) when
  you need a concrete low-cardinality log label model.
- Use [Log retention and access control](../logging/retention-and-access-control.md)
  before you expose audit logs to dashboards or search.
