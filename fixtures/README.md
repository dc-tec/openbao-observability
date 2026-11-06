# Fixtures

Captured OpenBao metrics and log fixtures validate contracts and generated
artifacts against observed OpenBao behavior.

Generated captures live under `fixtures/captured/` and are ignored by Git by
default because they contain timestamps, generated cluster IDs, and local
command output. Run `make fixtures-openbao` to regenerate them.

The HA/Raft capture includes production-like activity for root namespace
operations, one child namespace, one minimal nested namespace, database lease
lookup/renew/revoke behavior, audit logs, and all-node metrics.

The workload also lets a short-lived token expire naturally. Verification
requires expiration queue gauges, a dispatched-job wait observation, and the
Raft write/apply summaries on the active node. This checks metric shape and
availability; it does not reproduce a production-sized backlog.

Synthetic rule fixtures under `internal/rules/testdata/` exercise backlog
growth, draining with persistent latency, recovery, missing data, leader
transitions, summary counter resets, and both metric prefixes. Run them with
`make validate-generated`.

CI runs the capture and verification for OpenBao 2.6.3, 2.7.0, and 2.7.2. Release
scenarios check malformed audit input, canonical policy paths, authorization
errors, and the seal lifecycle. Seal captures use a separate Shamir-sealed Raft
node and five-second telemetry retention. Keys and tokens remain local to the
fixture run. Version 2.7.0 also checks consistency headers on the read replica.
Lifecycle captures live under `lifecycle/` and do not contribute to the normal
workload metric matrix.
