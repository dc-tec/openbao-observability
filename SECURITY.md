# Security policy

## Supported versions

Security fixes target `main` and the latest tagged minor release. Older minor
release lines do not receive backports unless maintainers explicitly announce a
backport window.

| Version | Supported |
| ------- | --------- |
| `main` | Active development and security fixes. |
| Latest tagged minor | Security fixes after publication. |
| Older tagged minors | Not supported unless explicitly announced. |

## Report a vulnerability

Do not report vulnerabilities, leaked secrets, unsafe audit-log examples, or
credential exposure through a public issue.

Use GitHub private vulnerability reporting when it is enabled for this
repository. If private reporting is not available, open a minimal public issue
asking for a private maintainer contact and do not include exploit details,
secret values, audit payloads, or environment identifiers.

Useful reports include:

- affected files, generated artifacts, or examples;
- whether the issue affects local examples, generated packaging, docs, or
  runtime tooling;
- the impact on audit-log confidentiality, metric or log label exposure,
  credentials, or generated deployment resources;
- reproduction steps that do not include real secrets or customer data.

## Scope

Security-sensitive areas include audit-log handling, Loki label strategy,
generated alert rules, examples that include credentials, fixture capture,
Docker Compose, Kubernetes examples, and generated packaging artifacts.

The local Docker Compose stack uses deterministic credentials and HTTP for
local evaluation only. Do not treat those values as production guidance.

## Known dependency caveats

`make vulncheck` uses `.govulnignore` for reviewed vulnerability IDs that are
not reachable through this repository's shipped or documented behavior. Keep
entries narrow, document the rationale next to each ID, and remove them when an
upgrade or advisory correction makes the ignore unnecessary.

`GO-2026-5662` is ignored because it affects Prometheus web UI tooltip and
metrics-explorer rendering. This repository imports Prometheus parser and text
format packages for offline contract, rule, dashboard, and fixture validation;
it does not serve, proxy, or embed the Prometheus web UI surface covered by the
advisory.
