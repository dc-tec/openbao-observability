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

The default PostgreSQL fixture image is `postgres:18-alpine`, matching the
current Compose profile. Set `POSTGRES_IMAGE` to select a different fixture
image. Fixture capture creates disposable containers; it does not migrate
Compose volumes.

The separate **PostgreSQL 19 qualification** pull request workflow selects
OpenBao `2.7.3` and `postgres:19-alpine` without changing the compatibility
matrix above. It captures fresh fixtures and runs the existing observability
assertions. Its synthetic SQL test checks TCP authentication with credentials
issued by OpenBao, rejects an incorrect password, renews and revokes leases,
and checks that revoked credentials cannot authenticate.

The migration test pauses its disposable OpenBao server, dumps PostgreSQL 18
roles with `pg_dumpall --globals-only`, and dumps the synthetic application
database with `pg_dump -Fc --create`. It restores roles with
`psql -X -v ON_ERROR_STOP=1` and the database with
`pg_restore --exit-on-error --create` into a separate PostgreSQL 19 volume.
It verifies restored rows, role passwords, grants, lease operations, and new
credential issuance. The PostgreSQL 18 volume never attaches to PostgreSQL 19.
The harness removes only resources it creates.

This is a hosted synthetic qualification, not an operator procedure for
production databases or permission to upgrade an existing Compose volume.
Follow the PostgreSQL [major upgrade documentation](https://www.postgresql.org/docs/19/upgrading.html)
and [SQL dump documentation](https://www.postgresql.org/docs/19/backup-dump.html)
when planning a database migration. The workflow records resolved image IDs,
repository digests, versions, results, and dump hashes in its artifact. It does
not upload credentials, dumps, tokens, or captured audit payloads.
