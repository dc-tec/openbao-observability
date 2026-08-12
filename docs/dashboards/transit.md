# OpenBao Transit dashboard

Use this explainer to read the generated OpenBao Transit dashboard. It is for
operators who need focused audit-log visibility into Transit key management,
cryptographic operation paths, denied requests, and response errors.

## What this dashboard is for

Use the Transit dashboard when the secret engines and mounts dashboard,
security detections, or `OpenBaoTransitOperationErrors` alert points to
Transit-specific activity.

The dashboard answers these questions:

- Did Transit key management requests occur?
- Did encrypt, decrypt, rewrap, sign, verify, HMAC, random, hash, or datakey
  requests occur?
- Which Transit response entries include errors?
- Did policy denied responses affect Transit paths?
- Which OpenBao node handled the audited Transit activity?
- Do operational logs mention Transit, cryptographic, or key symptoms?

## What this dashboard is not for

Do not use this dashboard as the source of truth for Transit key inventory,
current key versions, deletion settings, or key configuration. Use OpenBao APIs
for current state.

Do not use this dashboard as a cryptographic correctness check for client
applications. It shows OpenBao-side audit and operational context.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | UID | Used for |
| ----------- | --- | -------- |
| Loki | `loki` | Transit audit request, response, and operational symptom streams. |

The dashboard does not use Prometheus panels yet. The current contract keeps
Transit visibility audit-log based until Transit source metrics are validated
from OpenBao fixtures.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Cluster | Textbox | `.*` | Selects the stable OpenBao cluster identity. |
| Kubernetes namespace | Textbox | `.*` | Selects the OpenBao workload namespace when multiple OpenBao instances share the same observability backend. |
| Request ID | Textbox | `.*` | Narrow the stream to one request ID or pattern. |
| Transit mount path | Textbox | `transit` | Select the Transit secrets engine mount path. |
| Operation | Custom | `.*` | Filter to `read`, `list`, `create`, `update`, or `delete`. |
| Node | Textbox | `.*` | Narrow the stream to one OpenBao node label. |

Treat textbox values as LogQL regular expressions. Use the mount path without a
trailing slash, such as `transit` or `payments-transit`.

## How to read key management activity

Key management panels filter `transit/keys` paths. These events can represent
key reads, key creation, rotation, import, tuning, deletion settings, or other
key configuration changes depending on the request operation and path suffix.

Confirm that key management activity matches approved change windows. Transit
key changes can affect application decrypt, verify, or rewrap behavior.

## How to read cryptographic activity

Cryptographic panels filter Transit operation paths such as `encrypt`,
`decrypt`, `rewrap`, `sign`, `verify`, `hmac`, `random`, `hash`, and `datakey`.

Use request volume to understand which application workflows are active. Treat
unexpected spikes as workload or security signals, then correlate them with
application release, traffic, policy, and auth activity.

## How to read response errors

Response error panels filter audit response entries where OpenBao includes an
error field for Transit paths. Errors can come from policy denials, invalid
payloads, key version constraints, disabled operations, deleted keys, or client
misuse.

Start with the response error stream, capture the request ID, and then inspect
the matching request entry in the audit investigation dashboard.

## Label safety

The dashboard parses request IDs, request paths, operations, and audit errors at
query time. It does not require Transit key names, request IDs, client
addresses, token accessors, or entity IDs as Loki labels.

Keep Transit key names and request paths out of shared Loki labels. Key names
can reveal application boundaries and cryptographic workflows.

## Common mistakes

- Treating audit activity as current Transit key inventory.
- Treating all permission denials as malicious before you check policy tests,
  application rollouts, and expected denied requests.
- Rotating a Transit key only to clear an alert.
- Deleting or changing key configuration before you understand decrypt,
  verify, or rewrap impact.
- Grouping dashboards by Transit key name or request path.

## Known limitations

- The dashboard assumes the default Transit mount path is `transit`.
- Custom mount paths need the `Transit mount path` variable to match the
  deployment.
- The dashboard depends on Loki retention for `log_stream="openbao.audit"`.
- The dashboard does not use Transit source metrics because the project has not
  validated a stable Transit metric contract.
- Audit logs show OpenBao request and response records, not client-side
  cryptographic success.

## What's next

- Use [Secret engine feature warnings](../runbooks/secret-engine-feature-warnings.md)
  when the Transit warning alert fires.
- Use [OpenBao secret engines and mounts dashboard](./secret-engines-mounts.md)
  when you need a broader mount and secret-engine view.
- Use [OpenBao audit investigation dashboard](./audit-investigation.md) to
  inspect a request ID across audit fields.
- Use [Security audit detections](../runbooks/security-audit-detections.md)
  when Transit errors appear with privileged configuration activity.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add Transit key names or paths to labels.

Source: OpenBao documents Transit behavior in the
[OpenBao Transit documentation][openbao-transit]. This page describes the
generated dashboard contract in `contracts/dashboards/openbao-transit.yaml`.

[openbao-transit]: https://openbao.org/docs/secrets/transit/
