# Run the Docker Compose stack

Use this how-to to run a local OpenBao Observability reference stack with
OpenBao, Prometheus, Loki, Grafana Alloy, and Grafana. The stack is for local
evaluation and contract validation.

> [!WARNING]
> This stack runs OpenBao in dev mode with a local root token. You must not use
> it for production, shared environments, or sensitive data.

## Before you begin

- Install Docker with Docker Compose.
- Run commands from the repository root.
- Generate the latest rule artifacts before you start the stack.

## Start the stack

1. Generate Prometheus Operator and native Prometheus rule files.

   ```shell
   make generate
   ```

2. Start the Compose services.

   ```shell
   make compose-up
   ```

3. Check that the services are running.

   ```shell
   docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml ps -a
   ```

   Expected services:

   - `openbao`
   - `openbao-init`
   - `prometheus`
   - `loki`
   - `alloy`
   - `grafana`

## Open the local endpoints

| Service | URL | Purpose |
| ------- | --- | ------- |
| OpenBao | `http://127.0.0.1:18200` | Local OpenBao dev server. |
| Prometheus | `http://127.0.0.1:19090` | Metrics, recording rules, and alerts. |
| Loki | `http://127.0.0.1:13100` | Local log backend. |
| Alloy | `http://127.0.0.1:12345` | Collector status UI. |
| Grafana | `http://127.0.0.1:13000` | Explore metrics and logs. |

Grafana uses `admin` / `admin` by default. Change the local password in
`examples/docker-compose/.env` when you need a different local credential.

## Verify the result

1. Check OpenBao health.

   ```shell
   curl -fsS http://127.0.0.1:18200/v1/sys/health
   ```

   Expected output includes:

   ```json
   {
     "initialized": true,
     "sealed": false,
     "version": "2.5.4"
   }
   ```

2. Check Prometheus readiness.

   ```shell
   curl -fsS http://127.0.0.1:19090/-/ready
   ```

   Expected output:

   ```text
   Prometheus Server is Ready.
   ```

3. Check the OpenBao scrape target.

   ```shell
   curl -fsS 'http://127.0.0.1:19090/api/v1/query?query=up%7Bjob%3D%22openbao%22%7D'
   ```

   Expected output includes `"value"` with `1`.

4. Check an OpenBao metric.

   ```shell
   curl -fsS 'http://127.0.0.1:19090/api/v1/query?query=vault_core_active'
   ```

   Expected output includes the `vault_core_active` metric with value `1`.

5. Check Loki stream labels.

   ```shell
   curl -fsS http://127.0.0.1:13100/loki/api/v1/label/log_stream/values
   ```

   Expected output includes:

   ```json
   ["openbao.audit","openbao.operational"]
   ```

6. Check Grafana health.

   ```shell
   curl -fsS -u admin:admin http://127.0.0.1:13000/api/health
   ```

   Expected output includes `"database": "ok"`.

## Query the data

In Grafana, use the provisioned `Prometheus` data source to run these PromQL
queries:

```promql
vault_core_active
```

```promql
openbao:core_active:sum
```

Use the provisioned `Loki` data source to run these LogQL queries:

```logql
{log_stream="openbao.operational"}
```

```logql
{log_stream="openbao.audit"}
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

Stop containers and remove named volumes:

```shell
make compose-reset
```

## Troubleshooting

### Prometheus has no OpenBao target

Regenerate the rule files and restart the stack.

```shell
make generate
make compose-down
make compose-up
```

Check the init container if Prometheus still does not start.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs openbao-init
```

### Loki has no OpenBao streams

Check that Alloy is tailing both OpenBao files.

```shell
docker compose --project-directory examples/docker-compose -f examples/docker-compose/compose.yaml logs alloy
```

The Alloy logs include `start tailing file` for the audit and operational log
files when collection is active.

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
- Use `contracts/alerts/critical.yaml` as the source of truth for local alert
  changes.

Source: OpenBao documents dev mode as insecure and ephemeral in the
[OpenBao dev server documentation][openbao-dev-server]. OpenBao documents the
Prometheus metrics endpoint in the
[OpenBao telemetry documentation][openbao-telemetry]. OpenBao documents
configuration-defined audit devices in the
[OpenBao declarative audit documentation][openbao-audit]. Grafana documents
local file collection in the
[Alloy file source documentation][alloy-file-source].

[alloy-file-source]: https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.file/
[openbao-audit]: https://openbao.org/docs/configuration/audit/
[openbao-dev-server]: https://openbao.org/docs/concepts/dev-server/
[openbao-telemetry]: https://openbao.org/docs/configuration/telemetry/
