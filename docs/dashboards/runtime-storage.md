# OpenBao runtime and storage dashboard

Use this explainer to read the generated OpenBao runtime and storage dashboard.
It is for operators who need to correlate request latency with storage barrier,
cache, mount table, Go runtime, and operational log signals.

## What this dashboard is for

Use the runtime and storage dashboard when the overview dashboard shows request
latency, token-check latency, runtime pressure, or storage-related log
symptoms.

The dashboard answers these questions:

- Are storage barrier reads, writes, lists, or deletes getting slower?
- Is the storage cache hit ratio changing?
- Are Go runtime memory and GC signals growing with request latency?
- Did mount table inventory change across bounded `type` and `local` labels?
- Do operational logs mention storage, barrier, cache, runtime, GC, or memory
  symptoms?

## What this dashboard is not for

Do not use this dashboard as a storage backend tuning guide. It shows
correlation signals that help you decide where to investigate next.

Do not use cache hit ratio, barrier latency, or runtime memory as standalone
incident proof. Interpret those signals with request latency, token-check
latency, HA/Raft health, platform metrics, and operational logs.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | Expected UID | Used for |
| ----------- | ------------ | -------- |
| Prometheus | `prometheus` | Normalized OpenBao runtime, barrier, cache, and mount table recording rules. |
| Loki | `loki` | Runtime and storage operational logs. |

Deploy the generated Prometheus recording rules before you rely on this
dashboard. The panels use normalized `openbao:` rules rather than raw
`vault_*` or `openbao_*` source metrics.

The generated dashboard exposes cluster, Kubernetes namespace, and scrape
profile selectors. Use them to select one OpenBao deployment before you
compare runtime and storage signals.

## Scrape profile assumptions

The dashboard works with the authenticated active-node scrape, but all-node
scraping gives better runtime and storage visibility across standby and Raft
follower nodes.

| Scrape profile | What works well | Limitation |
| -------------- | --------------- | ---------- |
| Authenticated active-node scrape | Active request, barrier, cache, and runtime behavior. | Standby and follower runtime state is limited. |
| Private all-node scrape | Per-node context when runtime or storage pressure differs by node. | Requires isolated metrics access and label review. |
| Local Docker Compose scrape | Reference-stack validation and dashboard development. | Not a production security model. |

## How to read the summary row

Start with request latency and token-check latency. They tell you whether users
are likely to feel the storage or runtime symptoms shown elsewhere.

Then compare:

| Panel | Healthy interpretation |
| ----- | ---------------------- |
| Barrier GET latency | Read-path latency stays near its normal baseline. |
| Barrier PUT latency | Write-path latency stays near its normal baseline. |
| Cache hit ratio | The value remains consistent for the workload. |
| Goroutines | The value changes with workload and returns toward baseline. |

Treat a single high value as a lead, not a root cause. Correlate it with the
time series panels and logs before you decide on remediation.

## How to read barrier panels

Barrier panels show storage barrier operation rates and average latency. Read
rates before latency. A latency increase during a rate increase can point to
load growth. A latency increase without rate growth can point to storage,
Raft, audit, CPU, memory, or backend dependency pressure.

GET and PUT latency are the most important first-pass signals. LIST and DELETE
latency help explain list-heavy clients, cleanup work, and backend-specific
behavior.

## How to read cache panels

Cache hit, miss, and write rates show cache activity. Cache hit ratio shows
the proportion of cache reads that OpenBao serves as hits.

A lower hit ratio is not automatically bad. New workloads, cold caches, mount
changes, or different request patterns can all change the ratio. Compare the
ratio with request latency, token-check latency, and barrier latency before you
act.

## How to read runtime panels

Runtime panels show Go memory and GC pressure:

- Allocated bytes show currently allocated heap memory.
- System bytes show memory obtained from the operating system.
- Heap objects show object count pressure.
- GC pause and GC run count show garbage collection behavior.

Growing memory with stable latency is usually a trend to watch. Growing memory
with rising request latency, higher token-check latency, or operational log
errors needs deeper investigation.

## How to read mount table panels

Mount table entries and size use bounded labels: `type` and `local`. They do
not expose mount paths.

Use these panels as inventory context. A change can explain new token, lease,
secret-engine, or auth activity, but it does not identify the changed mount by
itself. Use audit investigation and change history when you need path-level
detail.

## How to read runtime and storage logs

The log panel filters operational logs for storage, barrier, cache, runtime,
GC, and memory terms. Use it to correlate metric changes with server-side
messages.

Operational logs are troubleshooting context. They are not audit records.

## Known limitations

- Most panels need generated recording rules.
- Active-node scraping gives limited standby and follower visibility.
- Barrier and cache baselines are workload-specific.
- Mount table panels use bounded inventory labels and do not show mount paths.
- Runtime panels show Go process pressure, not container or node limits.
- Operational logs depend on `log_stream="openbao.operational"`.

## Related pages

- Use [OpenBao overview dashboard](./overview-dashboard.md) when you need the
  first triage view.
- Use [OpenBao HA/Raft dashboard](./ha-raft.md) when barrier or storage
  latency correlates with Raft symptoms.
- Use [OpenBao token and lease lifecycle dashboard](./token-lease-lifecycle.md)
  when token checks, leases, or revocation work change at the same time.
- Use [OpenBao database secrets dashboard](./database-secrets.md) when storage
  or runtime pressure correlates with database credential latency.
- Use [OpenBao secret engines and mounts dashboard](./secret-engines-mounts.md)
  when mount table changes need audit-based context.
- Use [Runtime and storage warnings](../runbooks/runtime-storage-warnings.md)
  when runtime, storage, cache, or mount table warning alerts fire.
- Use [Understanding OpenBao metrics](../metrics/understanding-openbao-metrics.md)
  to understand source metrics, recording rules, labels, and scrape profiles.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add mount, path, policy, or identity dimensions.

Source: OpenBao documents telemetry metric behavior in the
[OpenBao telemetry metrics overview][openbao-telemetry-metrics]. The generated
dashboard contract is
`contracts/dashboards/openbao-runtime-storage.yaml`.

[openbao-telemetry-metrics]: https://openbao.org/docs/internals/telemetry/metrics/
