# Splunk Enterprise trial validation

This record covers the draft Splunk profile tested on 6–7 November 2026. It is
an interface and calculation qualification for a disposable fixture, not a
production benchmark or a Splunk Cloud certification.

## Tested environment

| Component | Version or configuration |
| --------- | ------------------------ |
| Splunk Enterprise | 10.4.5, official image pinned by digest, download-trial license. |
| OpenTelemetry Collector Contrib | 0.164.0, image pinned by digest. |
| OpenBao | 2.7.2, three-node Raft fixture with synthetic auth and secret workloads. |
| Reference metrics backend | Prometheus 3.15.1 with repository recording rules. |
| Calculation oracle | Prometheus promtool 3.11.2 and the committed synthetic fixtures. |
| Runtime | AMD64 Linux, rootless Podman 5.8.9, Podman Compose 1.5.0. |
| Metric collection | Independent 15-second scrapes from the collector and Prometheus. |
| Browser | Brave 155.1.97.59 on macOS 26.5.2, native computer-use review. |
| External sources | Blackbox Exporter 0.29.1 and the repository archive health exporter, on the private fixture network. |

The Splunk runtime retains license and index data in named volumes. Service
ports remain on loopback. SSH commands perform validation on the test host.
The browser review used a temporary VPN proxy restricted to the review Mac.
The proxy and its firewall rule were removed after logout. SSH forwarding
configuration did not change.

## Verified behavior

- Audit, operational, completed-request, Kubernetes Event, and metrics data
  reach separate indexes.
- Source audit timestamps and emitting node names survive collection.
- Prometheus target identity is restored from OpenTelemetry resource attributes.
- Summary sums, counts, and source quantiles reach the metrics index with the
  names used by the generated queries.
- All nine synthetic audit searches pass, including missing caller fields,
  a request ID after 6,000 bytes of padding, and secret-output exclusion. These
  are baseline searches; additional audit paths have separate family fixtures.
- Seven native counter/regression cases match an independent PromQL oracle:
  linear growth, reset, zero and sparse counters, growing and draining queues,
  and zero backlog.
- Native topology cases preserve missing data, reject stale inactive queues,
  keep leader histories separate, weight latency by observations, suppress
  idle latency, and prefer cluster seal state over duplicate startup state.
- All 15 generated Studio views, their macros, and navigation are accepted
  through the Splunk API. All 289 panel searches execute with default input
  substitution. Returned line-chart rows contain `_time`; metric stats contain
  the selected `value` field when data is present.
- Thirty-seven native family fixtures check audit path and type selection,
  custom mounts, namespace isolation, operational-field projection, duplicate
  reporter reduction, KV inventory, 15- and 30-minute counter windows, weighted
  means and cache ratios across namespaces, missing data, PVC pairing, probe
  objectives, archive health, completed-request field exclusion, and both
  Kubernetes Event formats. These events remain in memory.
- Independent PromQL fixtures also cover the new counter windows, namespace
  selection, mount and KV deduplication, cache ratios, observed probe weighting,
  error-budget arithmetic, and PVC capacity pairing.
- Family fixtures caught and verified fixes for timestamps lost during metric
  aggregation, empty scalar aggregates, stat-shaped SLO chart results, and
  incorrect aggregation of namespace means or ratios.
- All eight selected live comparisons pass after a bounded six-minute steady
  fixture workload. Four gauges agree exactly. Request rate and generic
  request/login/token-check means meet the original 10% relative tolerance.
- A separate OpenBao 2.7.2 dev server emits eight completed-request records.
  All eight reach the completed index through the file receiver and HEC.
  None appear in the operational index. The isolated server is then removed;
  completed logging remains disabled on the three-node fixture.
- Two recorded Kubernetes Events reach the platform index through the file
  receiver and HEC: one Core v1 Event and one `events.k8s.io/v1` Event. This
  checks JSON delivery and normalization, without a live Kubernetes cluster.
- Replacing the collector with its persisted file offsets leaves the controlled
  completed/Event counts unchanged: eight/two after the first fixture run and
  16/four after a repeat run. Starting the external services twice also succeeds.
  These checks do not establish exactly-once delivery or crash recovery.
- A real HTTP health probe reaches the metrics index as `probe_success=1`.
  A missing-endpoint control reaches it as `probe_success=0`. The negative
  control is excluded by the dashboard's default probe-job filter.
- The collector writes audit data to a rotating local OTLP JSON file. An
  observer finds a complete log record, and the archive health exporter reports
  successful local-file readback through the metrics pipeline. This does not
  qualify a remote or immutable archive.
- Six in-memory backfill cases check complete and target-weighted 30-day
  availability, exhausted budget, and rejection of short, sparse, or stale
  history. No backfill is indexed, and no real 30-day history is claimed.

## Browser and permissions

All 15 views render in Brave with their 289 panel containers. No search parse
error or invalid visualization was observed. The overview shows one active
node and three unsealed nodes. Representative duration and ratio labels retain
their source units. Long descriptions and legends can be elided in the layout;
this review does not qualify every viewport or browser.

An audit path wildcard changes the result set. A filter value containing a
quote, backslash, and pipe remains a literal filter, returns zero matches, and
does not introduce a search command. A nonexistent metric target leaves the
active-node value absent; a genuine reported zero remains visible.

Temporary roles use explicit index allowlists without imported roles or
administrative or scheduling capabilities. The reader can retrieve all 15
views through the API and read audit data. Representative overview, runtime,
and SLO views render in its browser session without an Edit control. A dashboard
write through the API is rejected. A separate operator role cannot query
the audit, completed, `main`, or `_internal` indexes. The fixture uses a
16-job search quota and a 256 MB search-disk quota for these test roles;
production quotas need sizing for concurrent users.

Both roles, their accounts, and their temporary ACL changes are removed after
review. The reviewed Enterprise instance retains its original `admin` ACLs.
Generated packages allow the platform's `admin` and `sc_admin` roles. This checks
one Enterprise permission configuration, not an organization's role design.

One comparison immediately after a short burst exceeded that tolerance:
request rate was about 1.85885 requests/s in Splunk and 2.09852 requests/s in
Prometheus, a difference of 11.4%. The tolerance was not increased. The
steady-workload check passed after filling the comparison window. Independent
scrape timing and window boundaries therefore still need qualification for
bursty production traffic; the steady result does not establish equivalence
for every traffic pattern.

The trial workload exercises userpass login. It does not qualify LDAP-specific
behavior or reproduce a large production expiration backlog.

## Release packaging and tool checks

A fresh Splunk Enterprise 10.4.5 container has no app before installation. The
clean-install check installs only the versioned `0.3.0-dev` `.spl` archive through
the native app-installation API, without app or index seed mounts. It creates
the five reference indexes separately. All 15 views, 289 panels, macros,
navigation, 17 disabled alerts, and administrator ACLs are present. Representative
HEC metrics and audit data produce one active-node result and one audit event;
all 289 panel searches execute. Empty results for other panels are expected
because this isolated instance has only controlled data.

The 17 native alert source searches execute. Fifty-seven in-memory SPL checks
cover firing and recovery for every binding, missing inputs, incomplete hold
periods, healthy resets, sample gaps, stale data, target separation, and expiry
of security activity outside five minutes. Three lookup checks cover missing
expected targets, healthy expected targets, and deployment isolation. The lookup
returns to its empty packaged state after the checks.

The clean instance temporarily enables the packaged sealed-node alert with a
one-minute schedule and an administrator execution owner. Controlled HEC samples
produce a native Triggered Alert. Healthy samples then produce a completed
scheduled search with zero firing rows. The test disables the alert and restores
its five-minute default. This qualifies scheduled execution for one packaged
binding; it does not establish external notification delivery or resolved
notifications. The test removes its container, anonymous volumes, network, and
credential files. The existing trial instance remains running.

The reference archive excludes `.env`, `.runtime`, and Python cache files without
depending on Git metadata. A regression fixture checks these exclusions using
synthetic private state. The installable `.spl` archive contains the 15 generated
views, macros, navigation, 17 disabled native alerts, an empty expected-target
lookup, app metadata, and license. It stamps both app-version
fields without changing generated source files; repeated packaging with the
same version and timestamp produces identical bytes.

Local AppInspect 4.4.0 precert checks report 101 successes, 152 not-applicable
checks, one skipped check, and zero errors, failures, future failures, or
warnings. The skipped check requires an optional `app.manifest`; this app uses
`app.conf` identity metadata. This result is a local package check, not Cloud
vetting. The CI and release workflows repeat the pinned package check.

The development tools live under `hack/splunk/` and are excluded from release
archives. Thirteen offline tests cover credentials, API boundaries, ACL rollback,
readiness, archive readback, shared image pins, disposable cleanup failures, and
endpoint restoration. They pass with Python 3.10
and the workstation interpreter. Ruff and ShellCheck also pass. The app itself
has no Python or Bash installation dependency.

The collector image accepts the configuration with its native `validate` command
without starting Splunk. CI also checks the combined Compose configuration and
collector configuration without accepting Splunk trial terms or starting a
Splunk server.

## Remaining qualification

Raft peer count is absent from the captured source telemetry. Its views return
no data. The profile preserves this absence; it does not
substitute zero. Other metrics can disappear during idle windows, particularly
observation-based latency and expiration signals.

These areas remain unqualified:

- Other browsers, viewport sizes, time-picker and mount-filter combinations,
  and the intended deployment's non-admin roles and search quotas.
- Splunk Cloud app vetting and platform-specific deployment.
- The other 32 canonical alerts, external notification delivery, resolved
  notifications, production execution roles, and incident search capacity.
  The focused set's scheduled test does not qualify routing or paging.
- Live Kubernetes exporter and Event collection. Recorded Event ingestion and
  synthetic metric calculations do not qualify a cluster deployment.
- Remote archive delivery, immutable retention, completeness, and recovery.
- Thirty days of probe collection and retention. The trial retains seven days.
- Full numerical equivalence with the Grafana profile. The profile documents
  audit bucket, series limit, namespace identifier, freshness, PVC summary, and
  stat aggregation differences.
- Production retention, ingestion volume, search cost, delivery guarantees,
  high-cardinality behavior, and performance under incident load.
- End-to-end Splunk compatibility with OpenBao versions other than 2.7.2.

## Reproduce the checks

Follow [the Splunk development trial guide](../../hack/splunk/README.md). The rate and
regression inputs are in [metrics.json](../../internal/splunk/testdata/metrics.json),
with the independent PromQL oracle in
[metrics.test.yaml](../../internal/splunk/testdata/metrics.test.yaml). The
synthetic audit data is in [audit.ndjson](../../internal/splunk/testdata/audit.ndjson).
The additional family cases are in
[families.json](../../internal/splunk/testdata/families.json), executed by
[family_checks.py](../../hack/splunk/family_checks.py). Runtime row counts
and comparison results remain in ignored `.runtime/` files. The repeatable
collection, history, and access checks are documented in that example. Use the
[browser review checklist](./splunk-browser-review.md) when validating another
platform or role.
