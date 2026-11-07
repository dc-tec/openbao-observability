# Configure native Splunk alerts

The Splunk profile includes 17 native scheduled alerts for core health,
audit delivery failures, expiration backlog, and security investigation.
The [alert coverage manifest](../../generated/splunk/alert-coverage.json) accounts
for all 49 canonical alerts. The other 32 are unmapped.

The app installs every alert disabled, with no email, webhook, or paging action.
It does not create indexes, collection services, or a search execution account.

## Included alerts

| Alert | Condition | Required duration |
| ----- | --------- | ----------------- |
| `OpenBaoUnreachable` | Reported `up=0`, or an expected target has no samples. | 5 minutes |
| `OpenBaoSealedUnexpectedly` | Observed unsealed state is zero. | 2 minutes |
| `OpenBaoNoActiveNode` | Sum of observed active flags is zero. | 2 minutes |
| `OpenBaoMultipleActiveNodes` | Sum of observed active flags exceeds one. | 2 minutes |
| `OpenBaoRaftPeerCountLow` | Observed peer count is below three. | 2 minutes |
| `OpenBaoAutopilotUnhealthy` | Observed Autopilot health is zero. | 2 minutes |
| `OpenBaoAutopilotNodeUnhealthy` | A reporter observes an unhealthy Raft node. | 2 minutes |
| `OpenBaoAutopilotFailureToleranceLost` | Peer count is at least three and failure tolerance is below one. | 2 minutes |
| `OpenBaoAuditRequestFailures` | Audit request failure counter increases in five minutes. | Immediate |
| `OpenBaoAuditResponseFailures` | Audit response failure counter increases in five minutes. | Immediate |
| `OpenBaoIrrevocableLeasesPresent` | Irrevocable lease count is positive. | 10 minutes |
| `OpenBaoExpirationBacklogGrowing` | Active node queue is positive and its five-minute slope is positive. | 15 minutes |
| `OpenBaoExpirationBacklogWithHighLatency` | Active node queue is positive and its five-minute request or login mean exceeds 1,000 ms. | 15 minutes |
| `OpenBaoAuditConfigurationActivity` | Audited create, update, or delete on `sys/audit`. | Immediate |
| `OpenBaoRootPolicyMutation` | Audited create, update, or delete on the root policy path. | Immediate |
| `OpenBaoPrivilegedConfigurationMutation` | Audited create, update, or delete on policy, auth, or mount configuration paths. | Immediate |
| `OpenBaoHighRiskSystemPathActivity` | Audited activity on raw storage, plugins, Raft storage, or rotation paths. | Immediate |

Security searches inspect the preceding five minutes. They count separate audit
request and response entries. A match indicates activity to investigate; it does
not establish that an operation succeeded or was unauthorized. Result rows omit
caller identities, token identifiers, request payloads, and raw audit records.

## Configure scope and collection

1. Set `defaultCluster`, `defaultScrapeProfile`, and `alertLogHostPattern` in
   [the profile contract](../../contracts/profiles/splunk-enterprise.yaml).

   Scheduled searches use fixed scope rather than dashboard input tokens.
   Metric alerts select one literal cluster and scrape profile. Audit alerts
   aggregate the configured emitting-host pattern into one deployment result.
   Choose a pattern that identifies only that deployment. A logical OpenBao
   namespace is separate from this deployment scope.

2. Configure the source macros and index permissions as described in
   [the profile](./splunk-enterprise.md). Verify a 15-second metric scrape
   interval and source event timestamps. Missing metrics do not become zero.

3. Regenerate and package the app after changing the contract.

   ```shell
   make generate-splunk validate-splunk
   make release-splunk-app VERSION=0.0.0-dev
   ```

4. To detect a target that disappears entirely, populate
   `lookups/openbao_alert_targets.csv` in the deployed app.

   ```csv
   cluster,kubernetes_namespace,scrape_profile,instance
   production,secrets,all_nodes,bao-0.example.com:8200
   production,secrets,all_nodes,bao-1.example.com:8200
   production,secrets,all_nodes,bao-2.example.com:8200
   ```

   Match the metric dimensions exactly. Use `(none)` when metrics have no
   Kubernetes namespace. Manage this deployment-specific inventory through your
   app deployment process, preserve it on upgrade, and remove decommissioned
   targets. The package contains only the CSV header. Without inventory,
   `OpenBaoUnreachable` detects reported scrape failures, but cannot identify a
   target whose entire series disappears.

The other metric alerts require observations. Loss of their inputs resets their
required duration; it does not prove recovery. The unreachable alert detects
configured target absence. Collection and scheduler health need separate
platform monitoring. Active-node and backlog histories remain separate across
leader transitions.

## Configure execution and notification

1. Assign an execution owner through Splunk's knowledge-object permissions.
   Use an account with scheduled-search capability, app access, and read access
   to the required indexes and lookup. The package restricts app objects to
   `admin` and `sc_admin`; adapt permissions to your platform's role design.
2. Review each search's scope, runbook, severity, and query cost. Run the search
   against representative data and exercise a failure and recovery in staging.
3. Configure notification actions, recipients, credentials, and suppression
   through your Splunk deployment process. The default tracks native Triggered
   Alerts without sending notifications. With suppression disabled, a sustained
   condition can create a record on each matching evaluation.
4. Enable only the validated alerts. Check their next scheduled run and search
   history after enabling them. The qualification tool sets `disabled=false`,
   `is_scheduled=true`, and the cron schedule together when activating its test.
5. Verify notification delivery and ownership before connecting production paging.

The default cron schedule is `*/5 * * * *`. Each search ends 30 seconds before
dispatch to allow ingestion to complete. Detection can therefore occur up to
about five minutes plus ingestion and search delay after the required duration.
"Immediate" in the table means no additional persistence requirement; the
schedule still applies. Use a faster cadence only after sizing search capacity.

Persistence uses retrospective 15-second condition history, rather than
Prometheus's in-memory pending state. All buckets in the required duration must
show a failing condition. A previous observation can fill a gap of up to 30
seconds; longer gaps and unknown required inputs reset the duration. Enabling
an alert can fire on its first run when existing history already satisfies it.
Review these constants together if the scrape interval changes.

A healthy evaluation returns no firing rows. This stops new trigger records;
it does not emit a resolved notification or remove earlier Triggered Alerts.
The app retains trigger records for one day and search artifacts for one hour.
Configure your incident-management integration if it requires a separate
resolved event.

## Validation and remaining boundaries

See the [Enterprise validation record](./splunk-enterprise-validation.md) for
archive installation, native SPL, and scheduled firing/recovery evidence.
The focused set does not establish full Prometheus/Loki alert equivalence.
Missing-data semantics, retrospective persistence, and scheduling differ.
External notification delivery, production search capacity, execution roles,
and Splunk Cloud deployment still require qualification on the target platform.

Native bindings reference canonical alert IDs, severity, runbooks, and required
durations. A canonical expression digest prevents generation after an expression
change until a maintainer reviews and updates its native binding. Edit the
contract and generator; do not edit generated `savedsearches.conf` files.
