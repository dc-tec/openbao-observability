# OpenBao auth and identity dashboard

Use this explainer to read the generated OpenBao auth and identity dashboard.
It is for operators who need restricted audit-log visibility into
authentication, token, identity, userpass, and AppRole activity.

## What this dashboard is for

Use the auth and identity dashboard when you need to inspect audited activity
around auth methods, login paths, token lifecycle paths, and identity paths.

The dashboard answers these questions:

- How much audited auth and identity activity matches the current filters?
- Which login and token lifecycle events are visible?
- Did identity entities, aliases, groups, or OIDC paths change?
- Did auth method configuration change?
- Did userpass or AppRole activity change?
- Which auth or identity responses contain errors?

## What this dashboard is not for

Do not use this dashboard as an identity source of truth. It reads audit events
that Loki collected. It does not replace OpenBao identity APIs, auth method
configuration, policy review, or SIEM retention.

Do not grant broad access to this dashboard. Auth and identity audit metadata
can reveal user, machine, auth method, and access-pattern details.

## Required data source

The generated dashboard expects a Loki data source with UID `loki`. The panels
query audit logs collected with `log_stream="openbao.audit"` and a bounded
`node_id` label.

The dashboard does not need Prometheus for its current panels, even though the
contract includes the standard metrics data-source definition for consistency
with the dashboard generator.

## Investigation filters

The dashboard exposes these variables:

| Variable | Type | Default | Use |
| -------- | ---- | ------- | --- |
| Request ID | Textbox | `.*` | Narrow the stream to one request ID or request ID pattern. |
| Auth or identity path | Textbox | `.*` | Narrow the stream to an auth, token, or identity path pattern. |
| Operation | Custom | `.*` | Filter to `read`, `list`, `create`, `update`, or `delete`. |
| Node | Textbox | `.*` | Narrow the stream to one OpenBao node label. |

Treat textbox values as LogQL regular expressions. Escape special characters
when you need an exact match.

## How to read auth activity

The first row counts auth and identity events, login events, token lifecycle
events, and identity mutations over five minutes. Read these panels as volume
and scope indicators, not as proof of success.

Use response-error panels and event logs to understand whether matching
requests succeeded or failed.

## How to read identity mutations

Identity mutation panels filter audited create, update, and delete operations
under identity entity, alias, group, group-alias, and OIDC paths.

OpenBao identity can connect authentications from different auth methods to
entities and aliases. Changes to identity objects can affect how policies apply
to already authenticated clients. Treat unexpected identity mutations as
security-sensitive changes.

## How to read auth method changes

Auth method changes filter audited mutations under `sys/auth/`. These events
can represent auth method enablement, tuning, or disablement.

Confirm that each auth method change matches an approved change or automation.
Unexpected auth method changes can alter how users and machines gain access to
OpenBao.

## How to read userpass and AppRole activity

The userpass and AppRole panels focus on common human and machine auth
patterns in the reference stack.

Userpass events can represent user and configuration changes. AppRole events
can represent role, role ID, secret ID, and login activity. Compare spikes with
expected automation windows and application behavior.

## Label safety

The dashboard parses request IDs, paths, and operations at query time. It does
not require those fields as Loki labels.

Keep auth accessors, entity IDs, token accessors, user names, client
addresses, and request paths out of Loki labels unless your organization
approves the security and cardinality tradeoff.

## Common mistakes

- Treating audit event volume as successful authentication volume.
- Forgetting that a request ID filter is still active.
- Treating userpass or AppRole activity as suspicious without checking expected
  automation.
- Promoting auth fields to Loki labels for dashboard convenience.
- Giving broad operational viewers access to identity audit metadata.

## Known limitations

- The dashboard depends on Loki retention for `log_stream="openbao.audit"`.
- It cannot show audit records that were never collected by Loki.
- It does not replace OpenBao identity and auth method APIs.
- Token backend activity does not always carry identity information.
- It interprets fields based on the current audit JSON shape.

## What's next

- Use [OpenBao audit investigation dashboard](./audit-investigation.md) for
  request ID and path drilldown across all audit paths.
- Use [OpenBao token and lease lifecycle dashboard](./token-lease-lifecycle.md)
  when token lifecycle panels need metric context.
- Use [High-cardinality and label safety](../concepts/high-cardinality-and-label-safety.md)
  before you add auth or identity fields to labels.
- Use [Configure declarative audit devices](../audit/declarative-audit.md) to
  review audit collection and access boundaries.

Source: OpenBao documents authentication behavior in the
[OpenBao authentication documentation][openbao-auth]. OpenBao documents
identity entities, aliases, groups, and identity auditing in the
[OpenBao identity documentation][openbao-identity]. This page describes the
generated dashboard contract in
`contracts/dashboards/openbao-auth-identity.yaml`.

[openbao-auth]: https://openbao.org/docs/concepts/auth/
[openbao-identity]: https://openbao.org/docs/concepts/identity/
