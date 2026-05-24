# Contributing

Use this guide when you contribute docs, contracts, examples, generated
artifacts, validation code, or tooling to the OpenBao Observability reference
architecture. The project is contract-first: source contracts define the
portable architecture, and generated artifacts implement one tested profile.

## Contribution principles

- Start from verified OpenBao behavior, fixtures, or upstream documentation.
- Keep the reference architecture portable across observability platforms.
- Treat the Prometheus, Loki, Grafana, and Grafana Alloy stack as an
  implementation profile, not as the architecture boundary.
- Keep audit logs separate from operational logs and treat them as restricted
  security records.
- Keep labels low-cardinality and avoid request paths, secret paths, token
  accessors, entity identifiers, policies, and client addresses as labels.
- Edit source contracts before generated artifacts.
- Keep implementation plans and local notes out of user-facing documentation.

## Set up your environment

- Install Go.
- Install Docker with Docker Compose when you work on fixtures, the local
  stack, or live dashboard query validation.
- Run commands from the repository root.
- Put local-only planning notes under `workstreams/` with a `.local.md`
  suffix. Git ignores those files.

## Change documentation

1. Follow the [documentation style guide](workstreams/docs-style-guide.md).
2. Keep each page focused on one type: how-to, runbook, reference, or
   explainer.
3. Use relative links for repository files.
4. Run documentation validation.

   ```shell
   make docs-verify
   ```

5. Build the Hugo documentation site when you change `docs/`, `website/`, or
   `hugo.toml`.

   ```shell
   make docs-build
   ```

## Change contracts or generated artifacts

1. Edit the relevant source contract under `contracts/`.
2. Regenerate artifacts.

   ```shell
   make generate
   ```

3. Validate contracts and generated artifacts.

   ```shell
   make contracts-verify
   make validate-generated
   ```

4. Review generated diffs before you open a pull request.

Generated files under `generated/` are outputs. Do not edit them by hand unless
you are repairing generator output as part of the same change.

## Change dashboards

1. Edit the dashboard contract under `contracts/dashboards/`.
2. Regenerate Grafana JSON.

   ```shell
   make generate
   ```

3. Validate dashboard queries against a running local stack when the query
   changes.

   ```shell
   make compose-up
   make validate-dashboard-queries
   ```

4. Update the dashboard documentation under `docs/dashboards/` when the user
   workflow changes.

## Change examples

1. Keep Docker Compose examples scoped to local evaluation and validation.
2. Keep Kubernetes examples adaptable to real labels, TLS, Prometheus Operator
   selectors, and network policies.
3. Validate Docker Compose configuration when you change the local stack.

   ```shell
   make compose-config
   ```

4. Do not add real tokens, certificates, hostnames, customer names, IP
   addresses, or audit payloads.

## Commit messages

Use Conventional Commits so release-please can build release notes and version
updates from the commit history.

```shell
git commit -s -m "feat: add dashboard contract"
```

Use the DCO signoff on every commit. The `-s` flag adds the required
`Signed-off-by` trailer.

## Validate your change

Run the narrow checks that match your change, then run the full verification
when you change contracts, generators, or generated artifacts.

```shell
make test-unit
make test-fixtures
make contracts-verify
make docs-verify
make docs-build
make validate-generated
```

Run the full verification before publishing generated artifacts.

```shell
make verify
```

## Publishing readiness

CI captures fresh OpenBao fixtures, runs the full repository verification, and
checks whitespace. Before publishing a release candidate or making the
repository public, run or verify the same gates locally:

```shell
make fixtures-openbao
make verify
git diff --check
```

Do not run `make validate-dashboard-queries` in baseline CI. It needs a running
local Compose stack and is a live-profile check for dashboard query behavior.

GitHub Actions workflows must pin third-party actions by commit SHA. Include a
comment with the source tag next to each pinned action so maintainers can audit
updates intentionally.

Release PRs are managed by release-please from Conventional Commits. Release
artifacts are draft by default until maintainers intentionally publish them.

## Pull request checklist

- [ ] The change preserves the reference architecture and implementation
      profile boundary.
- [ ] Source contracts, generated artifacts, and docs agree.
- [ ] Generated files were produced with `make generate`.
- [ ] Documentation follows `workstreams/docs-style-guide.md`.
- [ ] No secrets, live audit payloads, real customer identifiers, or sensitive
      environment values are included.
- [ ] Validation commands relevant to the change pass.
- [ ] Commits use Conventional Commits and include DCO signoff.

## Contribution license

Unless you explicitly state otherwise, contributions submitted to this project
are licensed under the [Apache License, Version 2.0](LICENSE), matching the
project license.
