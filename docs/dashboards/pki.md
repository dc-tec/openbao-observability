# OpenBao PKI dashboard

Use this explainer to read the generated OpenBao PKI dashboard. It is for
operators who need focused visibility into PKI issue and revoke metrics,
certificate lifecycle audit streams, issuer activity, and PKI response errors.

## What this dashboard is for

Use the PKI dashboard when the secret engines and mounts dashboard or a PKI
feature warning points to certificate lifecycle pressure.

The dashboard answers these questions:

- Are certificate issue or revoke operations active?
- Are PKI issue or revoke failures increasing?
- Is PKI issue or revoke latency elevated?
- Did issuer, root, intermediate, role, config, certificate, CRL, tidy, or
  revoke paths appear in audit logs?
- Which PKI responses include errors or permission denials?
- Which OpenBao node handled the audited PKI activity?

## What this dashboard is not for

Do not use this dashboard as the source of truth for current issuer state,
certificate inventory, CRL contents, role configuration, or certificate expiry.
Use OpenBao APIs and your certificate inventory for that.

Do not use this dashboard as a compliance archive. It uses Loki for exploration
and response, not authoritative long-term evidence.

## Required data sources

The generated dashboard expects these Grafana data sources:

| Data source | UID | Used for |
| ----------- | --- | -------- |
| Prometheus | `prometheus` | PKI issue and revoke rates, latency, and failure counters. |
| Loki | `loki` | PKI audit request and response streams. |

The metric panels use normalized `openbao:` recording rules. The log panels
parse audit JSON at query time and require `log_stream="openbao.audit"`.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Request ID | Textbox | `.*` | Narrow the stream to one request ID or pattern. |
| PKI mount path | Textbox | `pki` | Select the PKI secrets engine mount path. |
| Operation | Custom | `.*` | Filter to `read`, `list`, `create`, `update`, or `delete`. |
| Node | Textbox | `.*` | Narrow the stream to one OpenBao node label. |

Treat textbox values as LogQL regular expressions. Use the mount path without a
trailing slash, such as `pki` or `tenant-a-pki`.

## How to read PKI metrics

Issue and revoke rate panels show OpenBao-side PKI operation volume from
normalized recording rules. Latency panels show average issue and revoke
latency over 5 minutes.

Failure panels show failed issue or revoke operations when OpenBao emits the
corresponding counters. Treat failures as a signal to inspect role constraints,
issuer state, request payloads, storage pressure, Raft pressure, and client
workload changes.

The current metric contract validates root-namespace PKI metric families from
the default `pki` mount. Custom PKI mount names and namespace-derived PKI
metric families need fixture validation before you depend on them for
production alerts.

## How to read certificate lifecycle activity

Certificate lifecycle panels filter paths such as `issue`, `sign`,
`sign-verbatim`, `cert`, `certs`, `revoke`, `tidy`, and `crl`. These events can
represent certificate issuance, signing, reads, revocation, cleanup, and CRL
operations.

Use the request stream to correlate issue or revoke failures with the role,
request ID, operation, node, and nearby operational logs.

## How to read issuer and root activity

Issuer and root panels filter paths such as `issuer`, `issuers`, `root`, and
`intermediate`. Treat these events as higher-risk than ordinary certificate
issue reads because they can change trust chains or CA material.

Confirm issuer and root activity against approved changes. Use the security
audit detections runbook when issuer activity appears with privileged
configuration changes.

## Label safety

The dashboard parses request IDs, request paths, operations, and audit errors at
query time. It does not require certificate serial numbers, role names, request
IDs, client addresses, token accessors, or entity IDs as Loki labels.

Keep PKI role names, serial numbers, and request paths out of shared labels.
They can reveal internal service names, issuer topology, and certificate
lifecycle patterns.

## Common mistakes

- Treating issue rate as current certificate inventory.
- Treating missing PKI metrics as proof that the PKI engine is idle.
- Treating audit activity as a certificate archive.
- Grouping shared dashboards by role, serial number, request path, or client
  identity.
- Changing issuer or root configuration only to clear a latency warning.
- Ignoring storage and Raft symptoms during PKI latency investigation.

## Known limitations

- The dashboard assumes the default PKI mount path is `pki`.
- Custom mount paths need the `PKI mount path` variable to match the
  deployment.
- Root-namespace PKI metrics do not automatically cover namespace-derived PKI
  metric families.
- The dashboard depends on Loki retention for `log_stream="openbao.audit"`.
- The dashboard does not replace certificate expiry monitoring or CRL
  validation.

## What's next

- Use [Secret engine feature warnings](../runbooks/secret-engine-feature-warnings.md)
  when PKI warnings fire.
- Use [OpenBao secret engines and mounts dashboard](./secret-engines-mounts.md)
  when you need a broader mount and secret-engine view.
- Use [OpenBao audit investigation dashboard](./audit-investigation.md) to
  inspect a request ID across audit fields.
- Use [OpenBao runtime and storage dashboard](./runtime-storage.md) when PKI
  latency correlates with storage pressure.
- Use [OpenBao HA/Raft dashboard](./ha-raft.md) when PKI latency correlates
  with Raft or leader symptoms.

Source: OpenBao documents PKI behavior in the
[OpenBao PKI documentation][openbao-pki]. OpenBao documents telemetry metric
behavior in the [OpenBao telemetry metrics overview][openbao-telemetry].
This page describes the generated dashboard contract in
`contracts/dashboards/openbao-pki.yaml`.

[openbao-pki]: https://openbao.org/docs/secrets/pki/
[openbao-telemetry]: https://openbao.org/docs/internals/telemetry/metrics/
