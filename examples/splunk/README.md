# Collect OpenBao data for Splunk

Use the generated Splunk app with your existing Enterprise or Cloud Platform
deployment. Installing dashboards does not require Python or Bash helpers.
The app contains knowledge objects, not collector services or credentials.

1. Read the [profile configuration and collection contract](../../docs/implementation-profiles/splunk-enterprise.md).
2. Install the versioned `openbao_observability_<version>.spl` release artifact
   through your platform's app-installation process. Cloud installation still
   requires your administrator's review and platform vetting.
3. Create the profile's metrics and event indexes, or map the source macros to
   existing restricted indexes in `local/macros.conf`.
4. Collect OpenBao metrics and logs with your collector. Preserve source units,
   timestamps, deployment labels, and the separation between audit and
   operational records. Configure verified TLS and a dedicated HEC credential.
5. Apply your app and index permissions, then run the
   [browser review](../../docs/implementation-profiles/splunk-browser-review.md).
6. Configure the [native alert scope and execution owner](../../docs/implementation-profiles/splunk-alerts.md).
   The 17 packaged alerts install disabled and have no outbound actions.

The files in this directory configure a disposable Compose evaluation profile.
They use local HTTP endpoints, fixture credentials, and short retention. Adapt
the collection model, not these security settings, for production.

The [development trial guide](../../hack/splunk/README.md) explains how to run
the fixture and repeat native SPL checks. Those tools use Python's standard
library for HTTP and JSON and Bash for SSH/container orchestration. They are
development tools and are excluded from release artifacts.

Build the app from a source checkout when a release asset is not available:

```shell
make generate-splunk release-splunk-app VERSION=0.0.0-dev SOURCE_DATE_EPOCH=0
```

The package has a stable `openbao_observability` root directory and includes the
release version in `default/app.conf`. Packaging does not modify generated files
or include trial configuration, credentials, or indexed data.
