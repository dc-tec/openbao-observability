# Review the Splunk dashboards in a browser

Use this checklist after installing the generated app on the test instance.
API checks validate search execution and view definitions. They do not validate
browser rendering, token substitution, or access for a non-admin role.

The [validation record](./splunk-enterprise-validation.md#browser-and-permissions)
records the completed Brave/macOS review of all 15 views, literal filter inputs,
missing-target behavior, and representative read-only views. Repeat this
checklist on the intended platform; that review does not cover every control,
browser, viewport, or role.

## Open the app

1. Open Splunk Web on the test host at `http://127.0.0.1:18000`.
2. Sign in with the trial account. Read its password from the ignored `.env`
   file on that host; do not copy credentials into screenshots or review notes.
3. Open **OpenBao observability**. Confirm that navigation includes all 15
   families listed in the [profile coverage](./splunk-enterprise.md).
4. Open each view. Check that it loads without an invalid visualization, search
   parse error, clipped label, overlapping panel, or unusable horizontal layout.

The SSH-hosted trial binds ports to loopback. Browser access from another
machine requires an authorized access route. The helper does not change SSH
forwarding or firewall configuration.

## Check values and inputs

Run the bounded scenario and workload from the
[trial validation procedure](../../hack/splunk/README.md#run-validation)
when current feature values are needed. Idle summaries can expire from OpenBao
telemetry. Historical charts and current stats can therefore differ in data
availability.

- Select the `compose` cluster and `all_nodes` scrape profile for OpenBao
  metrics. Check that the overview shows one active node and three unsealed
  nodes when the fixture is healthy.
- Check that metric stats display the `value` field. Durations must retain their
  source unit: milliseconds for OpenBao request and storage summaries, seconds
  for probes, and nanoseconds for GC pause metrics. Ratios labelled `ratio (0-1)`
  must not appear as percentages multiplied by 100.
- Change the time picker. Audit counts and charts must change. Fixed-window
  availability, burn-rate, and inventory stats retain their stated trailing
  window to `now`; read each panel description before comparing values.
- Filter audit entries by request ID, request path, logical namespace, operation, and host.
  Check that each input affects the intended searches. Operational logs use the
  host input and must not depend on audit request fields.
- Change a database, PKI, or Transit mount-path wildcard. Check that the result
  follows that mount. Database lease-management paths use a separate source
  binding with the reference mount name. Logical OpenBao namespaces and Kubernetes namespaces
  must remain separate inputs.
- Enter values containing a quote, backslash, and pipe character into text
  inputs. They must remain quoted search values and must not introduce an SPL
  command. Restore the defaults after this check.
- Select a nonexistent metric target. The metric panels must show no data,
  rather than a healthy state or a fabricated zero. A real zero remains valid
  when the source reports it. An idle latency window can have no value.
- Check the metadata tables. They must omit raw audit payloads, credentials,
  token accessors, and secret request or response data. Error and operational
  message text can still be sensitive.

The trial includes a health probe and local-file archive health exporter.
Kubernetes metrics still need external exporters. The `log-delivery` check adds
real completed-request records from a disposable server and recorded Event JSON;
it does not deploy Kubernetes. The trial's short retention cannot populate a
complete 30-day availability window. The availability and remaining-budget
stats therefore remain unknown even while current probe charts have data.

## Record results

Record the Splunk version, view ID, panel title, selected inputs, and observed
behavior for each finding. Separate rendering problems, incorrect values,
missing source data, and permission failures. Exclude credentials and customer
data from screenshots.

Before broader use, repeat access checks with the intended non-admin role and
index permissions. Dashboard filters do not enforce authorization. Update the
[validation record](./splunk-enterprise-validation.md) only after observing the
corresponding result.
