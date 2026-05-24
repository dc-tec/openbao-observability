# Run the Docker Compose stack

Use this how-to to run a local OpenBao Observability reference stack with a
three-node OpenBao Raft cluster, PostgreSQL, Prometheus, Loki, Grafana Alloy,
and Grafana. The stack is for local evaluation and contract validation.

> [!WARNING]
> This stack uses HTTP, a local static seal key, deterministic local credentials,
> and unauthenticated metrics access inside the Compose network. You must not
> use it for production, shared environments, or sensitive data.

## Before you begin

- Install Docker with Docker Compose.
- Run commands from the repository root.
- Generate the latest rule artifacts before you start the stack.
- Reset the Compose volumes when you switch from an older single-node stack.

## Start the stack

1. Generate Prometheus Operator and native Prometheus rule files.

   ```shell
   make generate
   ```

2. Remove old local Compose volumes when you change the OpenBao topology.

   ```shell
   make compose-reset
   ```

3. Start the Compose services.

   ```shell
   make compose-up
   ```

4. Check that the services are running.

   ```shell
   docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml ps -a
   ```

   Expected services:

   - `openbao-seal-init`
   - `openbao-node0`
   - `openbao-node1`
   - `openbao-node2`
   - `postgres`
   - `prometheus`
   - `loki`
   - `alloy`
   - `grafana`

## Open the local endpoints

| Service | URL | Purpose |
| ------- | --- | ------- |
| OpenBao node 0 | `http://127.0.0.1:18200` | Raft bootstrap node and expected active node. |
| OpenBao node 1 | `http://127.0.0.1:18201` | Raft follower. |
| OpenBao node 2 | `http://127.0.0.1:18202` | Raft follower. |
| PostgreSQL | `127.0.0.1:15432` | Dynamic database secret backend for local lease activity. |
| Prometheus | `http://127.0.0.1:19090` | Metrics, recording rules, and alerts. |
| Loki | `http://127.0.0.1:13100` | Local log backend. |
| Alloy | `http://127.0.0.1:12345` | Collector status UI. |
| Grafana | `http://127.0.0.1:13000` | Explore metrics, logs, and dashboards. |

Grafana uses `admin` / `admin` by default. Change the local password in
`examples/docker-compose/.env` when you need a different local credential.
The stack provisions the generated `OpenBao overview` dashboard in the
`OpenBao` folder. It also provisions the generated `OpenBao HA/Raft`,
`OpenBao audit overview`, `OpenBao operational logs`, and
`OpenBao audit investigation`, `OpenBao auth and identity`, and
`OpenBao token and lease lifecycle`, and `OpenBao secret engines and mounts`
dashboards.

## Understand the local OpenBao setup

The stack starts `openbao-seal-init` first. That one-shot container writes a
32-byte static seal key into a named Docker volume if the key does not already
exist.

`openbao-node0` starts next and performs self-initialization. Only node 0 has
an `initialize` block. The self-initialization creates:

- the `userpass` auth method,
- the `approle` auth method,
- a KV v2 `secret/` mount,
- a database secrets `database/` mount backed by the local PostgreSQL service,
- a Transit `transit/` mount and local `payments` key,
- a local `compose-admin` policy,
- local `app-reader`, `app-writer`, and `identity-auditor` policies,
- a local `openbao-metrics` policy,
- the `demo-admin`, `demo-reader`, and `demo-writer` users,
- an `observability-app` AppRole and secret ID,
- a PostgreSQL `readonly` dynamic credential role,
- demo identity entities, entity aliases, and internal identity groups,
- a sample `secret/data/apps/payments/api` secret, and
- a deterministic local metrics token named
  `openbao-observability-metrics-token`.

`openbao-node1` and `openbao-node2` use `retry_join` to join node 0. They do
not initialize OpenBao, and they unseal through the shared local static seal
key.

Prometheus scrapes all three OpenBao nodes. The local OpenBao listener enables
`unauthenticated_metrics_access` so standby nodes expose metrics in this local
profile. Use a private metrics-only listener or equivalent network controls for
production all-node scraping.

## Verify the result

1. Check OpenBao health on each node.

   ```shell
   curl -sS http://127.0.0.1:18200/v1/sys/health
   curl -sS http://127.0.0.1:18201/v1/sys/health
   curl -sS http://127.0.0.1:18202/v1/sys/health
   ```

   This check does not use `-f` because standby nodes return a standby health
   status code while still reporting an initialized and unsealed node.

   Expected output includes:

   ```json
   {
     "initialized": true,
     "sealed": false,
     "version": "2.5.4"
   }
   ```

2. Log in with the local demo user.

   ```shell
   export BAO_ADDR=http://127.0.0.1:18200
   bao login -method=userpass username=demo-admin password=openbao-observability
   ```

3. Inspect the local auth and identity demo data.

   ```shell
   bao auth list
   bao list identity/entity/name
   bao list identity/group/name
   bao read auth/approle/role/observability-app/role-id
   ```

   Expected output includes the `userpass/` and `approle/` auth methods,
   `demo-admin`, `demo-reader`, `demo-writer`, `platform-team`, and
   `payments-team`.

4. Read a local dynamic database credential and inspect its lease.

   ```shell
   bao read database/creds/readonly
   bao list sys/leases/lookup/database/creds/readonly
   ```

   Expected output includes a lease ID under `database/creds/readonly/`.

5. Run the production-like fixture scenario.

   ```shell
   make fixtures-scenarios
   ```

   The scenario performs userpass and AppRole logins, KV activity, identity
   activity, token create/lookup/renew/revoke operations, database credential
   lease lookup/renew/revoke operations, Transit encrypt/decrypt operations,
   and expected denied requests.

6. Check Raft peers.

   ```shell
   bao operator raft list-peers
   ```

   Expected output includes `node0`, `node1`, and `node2` as voters.

7. Check Autopilot state.

   ```shell
   bao operator raft autopilot state
   ```

   Expected output includes a healthy cluster and a failure tolerance of `1`
   after Autopilot converges.

8. Check Prometheus readiness.

   ```shell
   curl -fsS http://127.0.0.1:19090/-/ready
   ```

   Expected output:

   ```text
   Prometheus Server is Ready.
   ```

9. Check the OpenBao scrape targets.

   ```shell
   curl -fsS -G http://127.0.0.1:19090/api/v1/query \
     --data-urlencode 'query=up{job="openbao"}'
   ```

   Expected output includes three `up` series with value `1`.

10. Check Raft recording rules.

   ```shell
   curl -fsS -G http://127.0.0.1:19090/api/v1/query \
     --data-urlencode 'query=openbao:raft_peers:max'

   curl -fsS -G http://127.0.0.1:19090/api/v1/query \
     --data-urlencode 'query=openbao:autopilot_failure_tolerance:max'
   ```

   Expected output includes peer count `3` and failure tolerance `1` after the
   rule evaluation interval passes.

11. Check Loki stream labels.

   ```shell
   curl -fsS http://127.0.0.1:13100/loki/api/v1/label/log_stream/values
   ```

   Expected output includes:

   ```json
   ["openbao.audit","openbao.operational"]
   ```

12. Check Grafana health.

   ```shell
   curl -fsS -u admin:admin http://127.0.0.1:13000/api/health
   ```

   Expected output includes `"database": "ok"`.

13. Validate dashboard queries against the local backends.

    ```shell
    make validate-dashboard-queries
    ```

    Expected output confirms that dashboard PromQL and LogQL queries validate
    against Prometheus and Loki.

## Query the data

In Grafana, open **Dashboards**, select the `OpenBao` folder, and open
`OpenBao overview`, `OpenBao HA/Raft`, `OpenBao audit overview`,
`OpenBao operational logs`, `OpenBao audit investigation`, or
`OpenBao auth and identity`, `OpenBao token and lease lifecycle`, or
`OpenBao secret engines and mounts`.

Use the provisioned `Prometheus` data source to run these PromQL queries:

```promql
up{job="openbao"}
```

```promql
openbao:core_in_flight_requests:max
```

```promql
openbao:core_handle_request:rate5m
```

```promql
openbao:core_handle_request:avg5m
```

```promql
openbao:core_handle_login_request:avg5m
```

```promql
openbao:core_check_token:avg5m
```

```promql
openbao:raft_peers:max
```

```promql
openbao:autopilot_node_healthy:min
```

```promql
openbao:token_count:max30m
```

```promql
openbao:expire_num_irrevocable_leases:max
```

Use the provisioned `Loki` data source to run these LogQL queries:

```logql
{log_stream="openbao.operational"}
```

```logql
{log_stream="openbao.operational"} |~ "\"@level\":\"(warn|error)\""
```

```logql
{log_stream="openbao.audit"}
```

Use this query when you need auth and identity audit events:

```logql
{log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"(auth/.*|sys/auth/.*|identity/.*)"
```

Use this query when you need token and lease lifecycle audit events:

```logql
{log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"(auth/token/.*|sys/leases/.*)"
```

Use this query when you need secret engine and mount activity:

```logql
{log_stream="openbao.audit"} | json request_path="request.path" | request_path=~"(secret|database|transit|pki|sys/mounts)(/.*)?"
```

Use this query when you need an audit request ID drilldown:

```logql
{log_stream="openbao.audit"} | json request_id="request.id" | request_id=~"<request_id>"
```

Use this query when you need audit event volume by Raft node:

```logql
sum by (node_id) (count_over_time({log_stream="openbao.audit"}[5m]))
```

Filter by Raft node when you need one node's logs:

```logql
{log_stream="openbao.operational", node_id="node1"}
```

## Change local settings

Create a local environment override when you need different ports, images, or
Grafana credentials.

```shell
cp examples/docker-compose/.env.example examples/docker-compose/.env
```

Then edit `examples/docker-compose/.env` and restart the stack.

```shell
make compose-down
make compose-up
```

The `.env` file is ignored by Git.

## Stop the stack

Stop containers and keep named volumes:

```shell
make compose-down
```

Stop containers and remove named volumes, including the local Raft data and
static seal key:

```shell
make compose-reset
```

## Troubleshooting

### Prometheus has no OpenBao targets

Regenerate the rule files and restart the stack.

```shell
make generate
make compose-down
make compose-up
```

Check the OpenBao node logs if Prometheus still does not scrape the nodes.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs openbao-node0
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs openbao-node1
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs openbao-node2
```

### The Raft cluster does not reach three voters

Check that node 0 initialized before node 1 and node 2 started.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs openbao-node0
```

Then inspect the follower logs for retry join errors.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs openbao-node1 openbao-node2
```

Run `make compose-reset` when a local Raft volume contains a failed or stale
test cluster.

### Loki has no OpenBao streams

Check that Alloy is tailing the per-node OpenBao files.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs alloy
```

The Alloy logs include `start tailing file` for audit and operational log files
when collection is active.

### Grafana has no data sources

Check Grafana provisioning logs.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs grafana
```

The stack mounts provisioning files from
`examples/docker-compose/grafana/provisioning`.

## What's next

- Inspect generated rule files in `generated/prometheus/`.
- Inspect Prometheus Operator rule artifacts in `generated/prometheusrules/`.
- Use `contracts/alerts/` as the source of truth for local alert changes.
- Use [OpenBao Raft and Autopilot health](./runbooks/raft-autopilot-health.md)
  when a Raft or Autopilot alert fires.

Source: OpenBao documents static seal configuration in the
[OpenBao static seal documentation][openbao-static-seal]. OpenBao documents
self-initialization in the
[OpenBao self-initialization documentation][openbao-self-init]. OpenBao
documents integrated storage and Raft join behavior in the
[OpenBao integrated storage documentation][openbao-integrated-storage]. OpenBao
documents the Prometheus metrics endpoint in the
[OpenBao telemetry documentation][openbao-telemetry]. OpenBao documents
configuration-defined audit devices in the
[OpenBao declarative audit documentation][openbao-audit]. Grafana documents
local file collection in the
[Alloy file source documentation][alloy-file-source].

[alloy-file-source]: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.file/
[openbao-audit]: https://openbao.org/docs/configuration/audit/
[openbao-integrated-storage]: https://openbao.org/docs/concepts/integrated-storage/
[openbao-self-init]: https://openbao.org/docs/configuration/self-init/
[openbao-static-seal]: https://openbao.org/docs/configuration/seal/static/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
