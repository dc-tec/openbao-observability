# Expected observability signals

Use an expectation marker when an optional signal is required in one
deployment. The generated alerts stay quiet when the marker is absent. They
report missing telemetry when the marker exists and the matching signal does
not exist.

The reference marker is this gauge:

```text
openbao_observability_signal_expected 1
```

Set the `signal` label to one of these values:

| Signal | Required identity labels | Observed metric |
| ------ | ------------------------ | --------------- |
| `kubernetes_pods` | `cluster`, `kubernetes_namespace` | `kube_pod_container_status_ready` |
| `synthetic_probe` | `cluster`, `job`, `instance` | `probe_success` |
| `log_collector` | `cluster`, `job`, `instance` | `up` for the collector scrape target |
| `audit_archive` | `cluster`, `environment`, `backend`, `pipeline` | `openbao_audit_archive_enabled` |

The identity labels on the expectation marker must equal the labels on the
observed metric. Add relabeling or recording rules when the source uses other
label names.

Use [expected-signals.example.yaml](./expected-signals.example.yaml) as an
example. Copy only the rules for signals that your deployment requires.
Replace all example label values before you load the rules.

Keep the expectation rules independent from the monitored target discovery.
For example, do not derive a synthetic probe expectation from
`probe_success`. If the source signal disappears, its expectation must remain.

The expectation rules run in Prometheus. They cannot report a Prometheus or
rule-evaluator outage. Monitor the evaluator from a separate system.
