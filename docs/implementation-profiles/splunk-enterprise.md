# Splunk Enterprise and Cloud Platform

Use this draft profile to evaluate OpenBao dashboards in Splunk Enterprise or
Splunk Cloud Platform with Dashboard Studio.
It targets the Dashboard Studio 9.4 definition format. It uses native SPL and
Splunk metrics indexes; it does not require Splunk Observability Cloud.

## Coverage and evidence

The profile generates 15 Dashboard Studio views with all 289 canonical
panels mapped to native searches:

| Family | Mapped panels | Required sources |
| ------ | ------------- | ---------------- |
| Overview | 24 | OpenBao metrics, audit JSON, operational JSON. |
| HA/Raft | 17 | OpenBao metrics, operational JSON. |
| Audit overview | 18 | OpenBao metrics, audit JSON, completed-request JSON, archive health metrics. |
| Audit investigation | 17 | Audit JSON. |
| Auth and identity | 16 | Audit JSON. |
| Token and lease lifecycle | 21 | OpenBao metrics, audit JSON. |
| Database secrets | 19 | OpenBao metrics, audit JSON. |
| Transit | 16 | Audit JSON, operational JSON. |
| PKI | 18 | OpenBao metrics, audit JSON. |
| Secret engines and mounts | 34 | OpenBao metrics, audit JSON. |
| Runtime and storage | 34 | OpenBao metrics, operational JSON. |
| Namespaces and scale | 19 | OpenBao metrics, audit JSON, operational JSON. |
| Kubernetes platform | 13 | kube-state-metrics, kubelet/cAdvisor, scrape health, Kubernetes Event JSON. |
| SLO and availability | 15 | External probes, OpenBao metrics, operational JSON. |
| Operational logs | 8 | OpenBao operational JSON. |

The [coverage manifest](../../generated/splunk/coverage.json) lists bindings,
unavailable panels, and required sources. `mapped` describes search coverage;
it does not establish collection, visual rendering, or production equivalence.
Completed-request and Kubernetes Event log adapters have collection checks.
A [focused native alert set](./splunk-alerts.md) covers 17 of the 49 canonical
alerts. The other 32 remain unmapped. Alerts install disabled and require an
execution owner, deployment scope, and operator-defined routing before use.
The [trial validation record](./splunk-enterprise-validation.md) states the
versions, checks, and limits of the current evidence.

The bindings map operator questions and disclose calculation differences. Audit counts use
the selected range; some Grafana audit panels use fixed rolling windows.
Metric rates and means use rolling five-minute windows. Counter increases use
five, 15, or 30 minutes as described in each panel. Extrapolated increases can
be fractional. Independently timed
scrapes and time buckets can produce different numbers near window boundaries.
A matching calculation fixture does not establish equivalence for every
production dataset.

## Before you begin

- Use a licensed test instance with Dashboard Studio before production adoption.
- Collect Prometheus-format OpenBao telemetry with the OpenTelemetry Collector.
- Use a metrics index for telemetry and separate event indexes for operational,
  audit, completed-request, and Kubernetes Event logs. Restrict access to audit
  and completed-request data.
- Preserve the emitting node, deployment identity, event timestamps, and source
  metric units. Do not scrape several nodes through one load-balanced target.
- Choose one cluster and scrape profile for single-value metric panels. The
  example defaults are `compose` and `all_nodes`. Chart series retain cluster,
  scrape profile, and target or peer identity.

## Collection model

The metrics path is OpenBao's `/v1/sys/metrics?format=prometheus` endpoint,
OpenTelemetry's Prometheus receiver, the Splunk HEC exporter, and a Splunk
metrics index. A Prometheus server is only needed as the comparison reference
in the test stack. The freshness guard is 45 seconds for a 15-second scrape
interval; review both settings together if you change the interval.

The collector does not execute this repository's Prometheus recording rules.
The Splunk contract defines separate SPL calculations. Counters need reset
correction; latency means need observation weighting. Missing metrics and idle
latency windows remain absent. Do not convert them to zero.

The example restores the Prometheus `instance` and `job` labels from
OpenTelemetry resource attributes before export. It also preserves the source
seal-state label so the queries can prefer cluster-labelled state over startup
duplicates. Summary sums and counts retain their original units. Summary
quantiles are exported under names such as `_0.99` and remain per source series.

Expiration queues use the active flag from the same target and time bucket.
A leader transition does not join independent backlog histories. Backlog slope
measures net change, including arrivals; it is not revocation throughput.
Dispatch-wait means describe jobs that reached dispatch, not jobs still waiting.

## Additional data sources and query semantics

The trial collector includes the OpenBao metric families used by the bindings,
a health probe, a failed-probe control, and a local-file archive health exporter.
It does not deploy Kubernetes exporters. Add separate Prometheus receiver jobs
for other sources. Keep the
original metric names and exporter dimensions, restore `instance` and `job`,
and assign a bounded `cluster` label. Use `scrape_profile=external` to identify
these jobs. External panels do not apply the OpenBao scrape-profile input.

- Kubernetes panels require the metric names in the Kubernetes dashboard
  contract. Normalize the exporter's `namespace` label to
  `kubernetes_namespace`. The binding also accepts `namespace` as a fallback.
  Pair each PVC's available and capacity values before calculating its free
  percentage; the summary shows the minimum percentage across matching claims.
  This differs from the Grafana summary's ratio of independently selected minima
  when several claims match.
- Probe panels require `probe_success` and `probe_duration_seconds` with bounded
  `job`, `instance`, and `cluster` labels. The objective input defaults to
  `0.999`. Values outside `0 < objective < 1` produce no result. Availability
  weights targets equally after averaging each target's observed samples.
  Missing probe samples are not successes. The 30-day availability and remaining
  budget stats require every selected source series to have at least 171,072
  observations (99% of 15-second samples), its first sample within 30 seconds
  of the window start, and its latest sample within 45 seconds of `now`.
  Incomplete history returns no result. Review these constants if the scrape
  interval changes. Target labels must remain stable across this window.
  The trial retains seven days, so real 30-day collection remains unqualified;
  in-memory backfill checks exercise the guard and arithmetic.
- Archive panels require the separate
  [archive health exporter contract](../../examples/audit-archive-health/README.md).
  The trial writes audit records to a rotating local OTLP JSON file. Its observer
  checks for a complete log record and uses the file modification time as the
  readback time. This checks the local path through the collector and exporter.
  It does not qualify remote object storage, immutable retention, or archive
  completeness.
- Operational panels expect OpenBao JSON logs with `@level`, `@module`, and
  `@message`. Collect the source `@timestamp` as event time. The macro projects
  only timestamp, host, severity, module, and message; message text can still
  contain sensitive information.
- Completed-request panels accept OpenBao JSON with `@message=completed_request`.
  The normalization exposes only timestamp, host, and event kind, and removes
  `_raw`. Enable `log_requests_level` only after reviewing the upstream logging
  configuration and data-access policy. The qualification helper enables it
  on a separate disposable server, then removes that server.
- Kubernetes Event panels accept Core v1 and `events.k8s.io/v1` JSON, either
  directly or under an `object` envelope with a bounded root `cluster` value.
  They normalize `involvedObject`/`regarding` and `message`/`note`, and project
  selected metadata and message fields. Recorded Event delivery is tested;
  a live Kubernetes Event collector and exporter deployment remain unqualified.

Stat means weight observations across the selected namespaces and targets.
Stat counts and rates total the selected series. Runtime goroutine stats total
the selected targets; the Grafana reference can display a separate stat per
target. Use the instance input when comparing a specific node.

Filters use quoted wildcard values. Logical OpenBao namespaces and Kubernetes
namespaces remain separate. A missing logical metric namespace is labelled
`(root)`; a missing Kubernetes namespace is labelled `(none)`. Audit namespace
filters use `request.namespace.path`, while some Grafana investigations use the
namespace ID. Database, PKI, and Transit views include a mount-path wildcard.
The combined engine view and database lease-management paths use the reference
mount names in their source bindings. Lease identifiers sent only in request
bodies cannot be attributed by these path-only searches.

Audit stat counts use the selected range. Audit charts use disjoint five-minute
buckets; request/response balance uses ten-minute buckets. These charts do not
reproduce every overlapping LogQL range window. Metric charts retain 20 series
and do not claim the Grafana `topk` ranking semantics. Select a namespace or
source target to reduce the result set.

Fixed-window availability, error-budget, burn-rate, and inventory stats use their
stated trailing window to `now`, independent of the time picker. Metric charts
use the picker and need enough earlier samples to fill their rolling windows.
A short selected range can produce partial initial windows. Inventory stats use
a 30-minute maximum with the freshness guard. Idle latency and incomplete ratios
remain absent instead of becoming zero.

## Configure and generate

1. Edit [the profile contract](../../contracts/profiles/splunk-enterprise.yaml).

   Set `auditIndex`, `auditSourcetype`, `metricsIndex`, `metricsPrefix`,
   `defaultCluster`, `defaultScrapeProfile`, `operationalIndex`, and
   `operationalSourcetype`, `completedIndex`, `completedSourcetype`,
   `platformIndex`, and `platformSourcetype`. Index names are literal.
   Supported metric prefixes are `vault` and `openbao`. Generation does not
   create indexes or enable collection.

2. Map the allowlisted audit fields to your JSON envelope.

   The defaults use `request.id`, `request.path`, `request.operation`,
   `request.namespace.path`, `auth.display_name`, `request.remote_address`,
   `type`, and `error`. Paths support dot-separated objects.

3. Generate and check the artifacts.

   ```shell
   make generate-splunk validate-splunk
   go test ./internal/contracts ./internal/splunk
   ```

   These commands check bindings and generated parity. They do not execute SPL.

## Run the trial profile

Follow [the development trial guide](../../hack/splunk/README.md).
It supports a Docker Compose workstation and a rootless Podman Linux host.
The trial uses synthetic secrets, loopback ports, short retention, and ignored
credentials. It is not a production deployment template.

For an existing instance, install the versioned `.spl` release asset through
your platform's app-installation process. It includes the release version in
`default/app.conf` and requires no trial scripts. To build it from source:

```shell
make generate-splunk release-splunk-app VERSION=0.0.0-dev SOURCE_DATE_EPOCH=0
```

The JSON and `.spl` files depend on the app's macros. Copying JSON alone does
not install those macros. Deployment-specific source changes can override
`openbao_audit_source`, `openbao_operational_source`, `openbao_completed_source`,
`openbao_platform_source`, and `openbao_metric_source` in `local/macros.conf`.
Splunk Cloud Platform can require administrator access and app vetting. This
repository does not claim Cloud vetting or Splunkbase certification.

## Verify the result

1. Check index ingestion, source timestamps, target identity, and collection health.
2. Run the synthetic audit, counter, backlog, topology, and weighted-mean checks.
3. Compare live values with Prometheus using the same evaluation end time.
4. Open all 15 Studio views on the target platform. Check units, selected
   fields, time controls, filters, and missing-data presentation.
5. Test filters containing quotes, backslashes, and pipe characters. Inputs use
   the documented `|s` quoting filter. The trial includes a Brave check; repeat
   it on the target platform.
6. Review app and index permissions with the intended role before sharing.
7. Configure and test [native alerts](./splunk-alerts.md) before enabling them.

Generated knowledge objects restrict access to the platform administrators:
`admin` on Enterprise and `sc_admin` on Cloud Platform. Dashboard filters do
not enforce authorization. The audit macro uses named `spath` extractions and
removes `_raw` before panel processing. Metadata tables omit credentials,
accessors, and request or response secret payloads. Source audit data and error
text still require restricted index access.

Missing caller names do not establish anonymous access. Audit request and
response entries are separate events, and duplicate ingestion inflates counts.
This profile does not infer HTTP status or client-observed latency from audit
errors. Use probes and OpenBao telemetry for service latency.

## Licensing and operational costs

Offline checks need no Splunk instance. Native SPL execution and Studio import
need a running instance with a suitable license. A standard Enterprise trial
provides 60 days and 500 MB/day; verify the effective license in the installation
you use. The Developer program offers a development license with a documented
10 GB/day allowance and six-month renewal cycle. Splunk Free lacks authentication
and alerting, so it is not the qualification target for this profile.

Bound metric families, dimensions, scrape interval, log retention, and search
time ranges. Measure indexed volume and search cost before expanding coverage.
Do not place request IDs, client IPs, token identifiers, or secret paths in
metric dimensions. Production also needs collector queue, retry, and delivery
health monitoring. Local file offsets persist across collector replacement;
this does not establish exactly-once log delivery.

## Platform references

- [OpenBao telemetry configuration](https://openbao.org/docs/configuration/telemetry/).
- [Prometheus receiver and unsupported recording rules](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.164.0/receiver/prometheusreceiver/README.md).
- [Splunk HEC metrics collection](https://help.splunk.com/en/splunk-observability-cloud/manage-data/splunk-distribution-of-the-opentelemetry-collector/get-started-with-the-splunk-distribution-of-the-opentelemetry-collector/collector-components/exporters/splunk-hec-exporter).
- [Dashboard Studio definition](https://help.splunk.com/en/splunk-enterprise/create-dashboards-and-reports/dashboard-studio/9.4/source-code-editor/what-is-a-dashboard-definition).
- [Named spath extraction](https://help.splunk.com/en/splunk-enterprise/search/spl-search-reference/10.6/search-commands/spath).
- [Install your license](https://help.splunk.com/en/splunk-enterprise/administer/admin-manual/10.6/start-splunk-enterprise-and-perform-initial-tasks/install-your-license).
- [Developer license types](https://help.splunk.com/en/splunk-enterprise/administer/admin-manual/10.4/configure-splunk-licenses/types-of-splunk-enterprise-licenses).
