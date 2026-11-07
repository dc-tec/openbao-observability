# Evaluate the Splunk profile

Use this disposable profile to collect OpenBao metrics, operational logs, and
audit logs in Splunk Enterprise and test native SPL. It uses the existing
three-node OpenBao fixture and Prometheus comparison backend.

These are development tools, not an installation requirement for the Splunk
app. Run them from a source checkout. Release archives exclude `hack/` and
private trial state. The [collection example](../../examples/splunk/README.md)
describes installation without these helpers.

## Tool maintenance

The HTTP, JSON, and calculation checks use Python 3.10 or later with no runtime
packages. Bash is limited to SSH and container launchers. A Bash rewrite would
add JSON tooling and escaping across shell, JSON, and SPL. Keep query semantics
in the contracts and Go generator rather than in either script language.

```shell
make install-splunk-tools
make test-splunk-tools lint-splunk-tools
```

Formatting and linting use the pinned Ruff development dependency and
ShellCheck. CI runs these checks with the repository tests. The offline tests
exercise credential preservation, file permissions, redirect rejection, error
redaction, readiness deadlines, archive readback, and ACL rollback. Native SPL
execution still requires the licensed disposable trial below.

Image defaults and digest pins live in the Compose files. The launchers read
those defaults and honor the corresponding image environment variables.
Dependabot checks the fixture images and pinned development tools.

## Check a release package

The app package requires neither Python nor these trial tools at installation.
From a source checkout, run the local vendor checks before release:

```shell
make generate-splunk release-artifacts VERSION=0.0.0-dev SOURCE_DATE_EPOCH=0
make install-splunk-appinspect
make inspect-splunk-app VERSION=0.0.0-dev SOURCE_DATE_EPOCH=0
```

AppInspect is a separate pinned development dependency. It requires `libmagic`
on the host; the current package is tested with Python 3.12 and 3.13. The gate
rejects errors, failures, future failures, and warnings. CI and release workflows
run it. Passing local AppInspect does not establish Splunk Cloud vetting or a
deployment qualification. Review the
[validation record](../../docs/implementation-profiles/splunk-enterprise-validation.md)
before making support claims.

## Before you begin

- Run commands from the repository root.
- Review the [profile and coverage boundary](../../docs/implementation-profiles/splunk-enterprise.md).
- Use Docker Compose locally, or an authorized AMD64 Linux test host with
  rootless Podman, subordinate UID/GID ranges, SSH, and rsync.
- The Linux helper can obtain Podman, Podman Compose, and Python through an
  existing Nix registry. It does not change the host's NixOS configuration.
- Review the [Splunk General Terms](https://www.splunk.com/en_us/legal/splunk-general-terms.html)
  and confirm that a trial is suitable for your development use.
- Use only synthetic data. The example uses HTTP OpenBao endpoints and accepts
  Splunk's self-signed certificate inside the test network and on loopback.
  Production deployments must verify TLS certificates and define their own
  access controls and credentials.

## Initialize credentials

```shell
python3 hack/splunk/local.py setup --accept-splunk-terms
make generate-splunk validate-splunk
```

Setup writes random credentials into ignored `examples/splunk/.env` with mode
`0600`. It preserves existing credentials. The license acceptance flags apply
only after you select the terms option. Do not commit license files, credentials,
private startup logs, or `.runtime/` artifacts.

## Start on a workstation

```shell
docker compose --env-file examples/splunk/.env \
  --project-directory examples/docker-compose \
  -f examples/docker-compose/compose.yaml -f examples/splunk/compose.yaml \
  up -d openbao-node0 openbao-node1 openbao-node2 prometheus splunk
```

Wait for readiness with `python3 hack/splunk/local.py wait`. Initial Splunk
provisioning can take several minutes. The official image is AMD64-only and
requires emulation on an ARM workstation. Once ready, create the profile indexes,
install the app, and start its internal probe, archive observer, and collector:

```shell
python3 hack/splunk/local.py indexes
python3 hack/splunk/local.py install
python3 hack/splunk/local.py external --engine docker
python3 hack/splunk/local.py collector --engine docker
```

Open Splunk Web at `http://127.0.0.1:18000` on the test machine. The username is
`admin`; read the generated password from the ignored `.env` file. Management
and HEC endpoints use loopback ports `18089` and `18088`.

## Start on an SSH test host

Replace `splunk-test` with an authorized SSH alias:

```shell
bash hack/splunk/remote.sh splunk-test up
bash hack/splunk/remote.sh splunk-test status
```

The helper copies source into `~/openbao-observability-splunk`, installs a
cross-compiled validation CLI, and runs rootless containers. It uses named
volumes for Splunk configuration, license state, indexed data, OpenBao data,
and collector file offsets. The Splunk seed directory is
`~/openbao-splunk-trial`.

The helper also builds and copies a Linux archive-health binary, waits for
Splunk, creates missing trial indexes, installs the app, and starts internal
external sources and collection. Existing remote credentials are preserved;
read the remote `.env` when its contents differ from the workstation's file.
Replacing the collector preserves its file-offset volume.

Enable systemd user lingering for the test account before starting the trial
if containers must survive SSH disconnects. The helper checks this prerequisite;
it does not change host persistence, SSH forwarding, or firewall rules. All published
ports bind to loopback. Splunk Web is therefore reachable from a browser on
the test host; a separate approved access route is needed for a browser on
another machine.

The container image policy is scoped to this profile's reference repositories
and rejects other repositories. Splunk and OpenTelemetry image digests are
pinned. The fixture's existing OpenBao, PostgreSQL, and Prometheus defaults
remain separate from production sizing or compatibility guarantees.

## Run validation

On the test host, run:

```shell
python3 hack/splunk/local.py install
python3 hack/splunk/local.py audit
python3 hack/splunk/local.py synthetic-metrics
python3 hack/splunk/local.py synthetic-topology
python3 hack/splunk/local.py synthetic-families
./openbao-observability fixtures scenario --output tmp/splunk-scenario.json
python3 hack/splunk/local.py workload
python3 hack/splunk/local.py panels
python3 hack/splunk/local.py parity
```

Use `go run ./cmd/openbao-observability` in place of the cross-compiled CLI on
a workstation. The bounded workload performs 90 fixture login, write, read,
and short-token cycles. It creates short-lived expiration activity without
attempting to reproduce the production incident's queue size.

`install` writes generated macros, navigation, and Studio views through the test
API. `audit` tests eight synthetic entries and nine baseline audit searches without
indexing those synthetic entries. `synthetic-metrics` tests counter resets,
zeros, sparse samples, and backlog regression. `synthetic-topology` tests active
selection, independent leader histories, missing queues, weighted latency, idle
windows, and duplicate seal-state sources. `synthetic-families` tests the new
family bindings with in-memory audit, operational, and metric fixtures. It checks
namespace isolation, reporter deduplication, inventory, weighted means, exporter
ratios, SLO objectives, and missing data. It does not index test events.

`panels` executes all 289 generated searches with their default input values and
configured fixed windows. It saves row counts in `.runtime/`.
No rows can mean an unavailable metric or an idle observation window; a search
that returns no rows is not automatically a successful data-availability check.
`parity` compares selected live values with Prometheus at a shared evaluation
end time. Gauges require exact agreement. Rates and means allow 10% relative
difference because the backends scrape independently every 15 seconds.

For the same checks over SSH after generating workload data:

```shell
bash hack/splunk/remote.sh splunk-test validate
```

The helper runs synthetic checks and panel/parity queries. Generate fresh
workload data first when you need numeric latency comparisons rather than idle
windows. Read the [validation record](../../docs/implementation-profiles/splunk-enterprise-validation.md)
for the tested platform and remaining limits. The helper also checks full-window
history guards and live external-source samples. Cloud vetting and native
scheduled execution remain outside that command. Use the clean installation
check below to qualify the packaged alerts.

## Test a clean release installation

Build a versioned archive, then run this command on the licensed test host:

```shell
make generate-splunk release-splunk-app VERSION=0.0.0-dev \
  SPLUNK_APP_PACKAGE=tmp/openbao_observability_clean.spl
python3 hack/splunk/local.py clean-install --engine podman \
  --package tmp/openbao_observability_clean.spl
```

Use `--engine docker` on a supported workstation. The test reuses the accepted
trial terms from `.env`, creates a separate container and network with random
credentials, and publishes only management and HEC ports on random loopback
ports. It leaves the existing trial instance running.

The test verifies that the app is absent, installs only the `.spl` archive
through the app-installation API, creates the five reference indexes, ingests
representative data, and executes all 289 panel searches. It checks that all
17 packaged alerts install disabled without outbound actions. Native fixtures
exercise failure, recovery, persistence, missing inputs, target separation,
security-window expiry, and expected-target inventory.

One packaged sealed-node alert is temporarily enabled with a one-minute
schedule in this disposable instance. Controlled HEC samples must produce a
native Triggered Alert, then a completed scheduled search with zero firing
rows after recovery. The test restores the five-minute schedule and disables
the alert. It does not test email, webhooks, paging, or resolved notifications.

The test removes its container, anonymous volumes, network, and credential
files on success or failure. It retains a private report or startup log under
`.runtime/`. Use `--keep` only to diagnose a failure; it also retains the
container and mode-0600 `private.json` and `credentials.env`. Remove those
resources after diagnosis. Do not publish these files or raw startup logs.

For predicate-only checks against an installed test app, run:

```shell
python3 hack/splunk/local.py synthetic-alerts
```

This command executes source queries and in-memory histories. It does not
enable scheduled searches or change the expected-target inventory. Review the
[native alert configuration](../../docs/implementation-profiles/splunk-alerts.md)
before enabling alerts on another deployment.

## Qualify collection and restricted access

Run these commands on the test host after indexes, external sources, and the
collector are ready. Use `--engine docker` on a workstation:

```shell
python3 hack/splunk/local.py log-delivery --engine podman
python3 hack/splunk/local.py offset-restart --engine podman
python3 hack/splunk/local.py verify-external
python3 hack/splunk/local.py history
python3 hack/splunk/local.py access
```

`log-delivery` starts a network-isolated disposable OpenBao server with completed
request logging, performs synthetic writes, and removes that server. It verifies
emitted/indexed counts and separation from operational logs. It also delivers
two recorded Kubernetes Events. It leaves bounded test files in `.runtime/logs`.
The three-node fixture's logging configuration does not change.

`offset-restart` replaces only the collector and checks that acknowledged test
records are not indexed again. It does not prove exactly-once delivery.
`verify-external` requires a healthy probe, a failed-probe control, and successful
local audit-file readback. These services use the internal container network;
they publish no additional host ports. The archive observer reads real OTLP
log records from `.runtime/archive`, with 10 MB rotation and one backup.
It does not qualify remote storage or immutable retention.

`history` generates six backfill cases in memory without indexing them. It
checks the 30-day history guard and target-weighted availability/budget
calculations. Real 30-day retention still requires a longer-running deployment.

`access` temporarily creates a reader and an operator role, applies scoped read
ACLs, and tests view reads, forbidden indexes, and dashboard write denial. It
restores the original ACLs and removes its accounts on success or failure.
To retain a successful reader session for browser review, run:

```shell
python3 hack/splunk/local.py access --keep-for-browser
```

The original ACLs and temporary credentials are saved in mode-0600
`.runtime/access-private.json`. After the browser review, run:

```shell
python3 hack/splunk/local.py access-cleanup
```

This restores the saved ACLs and removes both temporary roles and accounts.
Run cleanup even if a browser check fails. Do not share the private state file.
For Podman setup outside `remote.sh`, build the exporter first:

```shell
mkdir -p examples/splunk/.runtime/bin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -o examples/splunk/.runtime/bin/archive-health ./examples/audit-archive-health/exporter
python3 hack/splunk/local.py external --engine podman
```

## Validate an existing test instance

The Go audit query validator also accepts an authorized test instance:

```shell
export SPLUNK_URL=https://splunk-test.example.com:8089
export SPLUNK_CA_CERT=/path/to/private-ca.pem
make validate-splunk-queries
```

Supply `SPLUNK_TOKEN` through the environment or a secret manager. Bearer
authentication is the default; `SPLUNK_AUTH_SCHEME=Splunk` selects a session key.
The Go validator verifies TLS by default and refuses redirects. Its
`--insecure-loopback` option accepts a self-signed certificate only for a literal
loopback host. It cannot disable verification for remote endpoints.

## Stop and preserve the trial

For an SSH host:

```shell
bash hack/splunk/remote.sh splunk-test stop
```

For a workstation, run `python3 hack/splunk/local.py external-stop --engine
docker`, then use the same Compose files with `stop`. These actions
preserve named volumes and test files. Do not delete and recreate volumes to reset an expired
trial. Install an appropriate development license for continued use. Disable
user lingering separately only if no other user services require it.

## Review in the browser

Use the [browser review checklist](../../docs/implementation-profiles/splunk-browser-review.md)
for visual rendering and input behavior. API acceptance and search execution do
not establish those results. The trial collects health probes and local-file
archive health. Kubernetes metric exporters still require a separate target.
