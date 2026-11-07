"""Qualify native alert conditions, temporal windows, and scheduled recovery."""

import json
import time
import urllib.parse
from datetime import datetime

from family_checks import fixture_search

GAUGES = {
    "OpenBaoUnreachable": [("up", 0, 1)],
    "OpenBaoSealedUnexpectedly": [("vault_core_unsealed", 0, 1)],
    "OpenBaoNoActiveNode": [("vault_core_active", 0, 1)],
    "OpenBaoMultipleActiveNodes": [("vault_core_active", 2, 1)],
    "OpenBaoRaftPeerCountLow": [("vault_raft_peers", 2, 3)],
    "OpenBaoAutopilotUnhealthy": [("vault_autopilot_healthy", 0, 1)],
    "OpenBaoAutopilotNodeUnhealthy": [("vault_autopilot_node_healthy", 0, 1)],
    "OpenBaoAutopilotFailureToleranceLost": [("vault_raft_peers", 3, 3), ("vault_autopilot_failure_tolerance", 0, 1)],
    "OpenBaoIrrevocableLeasesPresent": [("vault_expire_num_irrevocable_leases", 1, 0)],
}
COUNTERS = {
    "OpenBaoAuditRequestFailures": "vault_audit_log_request_failure",
    "OpenBaoAuditResponseFailures": "vault_audit_log_response_failure",
}
SECURITY = {
    "OpenBaoAuditConfigurationActivity": "sys/audit/file",
    "OpenBaoRootPolicyMutation": "sys/policies/acl/root",
    "OpenBaoPrivilegedConfigurationMutation": "sys/auth/ldap",
    "OpenBaoHighRiskSystemPathActivity": "sys/storage/raft/snapshot",
}


def history(alert, bad=True):
    """Deterministic histories independent of the generated condition thresholds."""
    identifier = alert["id"]
    if identifier in SECURITY:
        return [
            {
                "host": "openbao-node0",
                "operation": "update" if bad else "read",
                "request_path": SECURITY[identifier] if bad else "secret/data/fixture",
                "time_offset": 0,
                "_raw": "FIXTURE_SECRET",
            }
        ]
    rows = []
    duration = alert["holdSeconds"] + (300 if identifier in COUNTERS or "Backlog" in identifier else 0) + 30
    for step, offset in enumerate(range(-duration, 1, 15)):
        values = [(metric, fail if bad else healthy) for metric, fail, healthy in GAUGES.get(identifier, [])]
        if identifier in COUNTERS:
            values = [(COUNTERS[identifier], step if bad else 0)]
        if "Backlog" in identifier:
            values = [("vault_core_active", 1), ("vault_expire_job_manager_total_jobs", 100 + step if bad else 0)]
            if "HighLatency" in identifier:
                values += [
                    ("vault_core_handle_request_count", step),
                    ("vault_core_handle_request_sum", step * (2000 if bad else 100)),
                ]
        for metric, value in values:
            rows.append(
                {
                    "metric_name": metric,
                    "value": value,
                    "time_offset": offset,
                    "_timeseries": "fixture/" + metric,
                    "cluster": "compose",
                    "scrape_profile": "all_nodes",
                    "instance": "fixture",
                    "job": "openbao",
                    "kubernetes_namespace": "(none)",
                    "seal_state_source": "cluster",
                    "_raw": "FIXTURE_SECRET",
                }
            )
    return rows


def execute(local, alert, rows):
    query = (local.ROOT / "generated/splunk/alerts" / (alert["id"] + ".spl")).read_text()
    source = (
        fixture_search(rows)
        + "\n| addinfo | eval bucket_time=floor(info_max_time/15)*15-15+coalesce(time_offset,0), _time=bucket_time"
    )
    query = source + query[query.index("\n") :]
    return local.search(query, earliest=alert["earliest"])


def run(local):
    coverage = json.loads((local.ROOT / "generated/splunk/alert-coverage.json").read_text())
    alerts = [alert for alert in coverage["alerts"] if alert["status"] == "mapped"]
    checks = []
    for alert in alerts:
        query = (local.ROOT / "generated/splunk/alerts" / (alert["id"] + ".spl")).read_text()
        local.search(query, earliest=alert["earliest"], latest="-30s")
        for bad, expected in [(True, 1), (False, 0)]:
            rows = execute(local, alert, history(alert, bad))
            label = alert["id"] + (" fires" if bad else " recovers")
            if len(rows) != expected or "FIXTURE_SECRET" in json.dumps(rows) or any("_raw" in row for row in rows):
                raise RuntimeError(label + ": incorrect result or private-field projection: " + str(rows))
            checks.append(label)
            print(label + ": passed", flush=True)
        if alert["id"] not in SECURITY:
            if execute(local, alert, []):
                raise RuntimeError(alert["id"] + ": absent input invented a condition")
            checks.append(alert["id"] + " missing input stays unknown")
        else:
            expired = [row | {"time_offset": -315} for row in history(alert)]
            if execute(local, alert, expired):
                raise RuntimeError(alert["id"] + ": activity outside the five-minute window fired")
            checks.append(alert["id"] + " old activity expires")
    sealed = next(alert for alert in alerts if alert["id"] == "OpenBaoSealedUnexpectedly")
    for label, rows, expected in [
        (
            "hold not yet complete",
            [row | {"value": 1 if row["time_offset"] < -105 else 0} for row in history(sealed)],
            0,
        ),
        (
            "healthy sample resets hold",
            [row | {"value": 1 if row["time_offset"] == -60 else 0} for row in history(sealed)],
            0,
        ),
        (
            "long sample gap resets hold",
            [row for row in history(sealed) if row["time_offset"] not in (-90, -75, -60)],
            0,
        ),
        ("one missed scrape is tolerated", [row for row in history(sealed) if row["time_offset"] != -60], 1),
        ("stale condition stays unknown", [row for row in history(sealed) if row["time_offset"] < -30], 0),
        (
            "unrelated target cannot complete a hold",
            [row | {"instance": "a" if row["time_offset"] < -60 else "b"} for row in history(sealed)],
            0,
        ),
    ]:
        if len(execute(local, sealed, rows)) != expected:
            raise RuntimeError(label + ": incorrect temporal result")
        checks.append(label)
        print(label + ": passed", flush=True)
    report = {
        "mapped_alerts": len(alerts),
        "checks": len(checks),
        "boundary": "In-memory predicate and persistence checks; scheduled execution is tested separately.",
    }
    print(json.dumps(report), flush=True)
    return report


def scheduled(local, state):
    """Exercise the packaged search with its native trigger and no external actions."""
    from clean_install import hec

    identifier = "OpenBaoSealedUnexpectedly"
    endpoint = "/servicesNS/nobody/openbao_observability/saved/searches/" + identifier
    auth = local.session()
    local.request(
        endpoint + "/acl",
        {"owner": "admin", "sharing": "app", "perms.read": "admin", "perms.write": "admin"},
        auth,
    )
    endpoint = endpoint.replace("/nobody/", "/admin/")
    local.request(endpoint, {"disabled": False, "is_scheduled": True, "cron_schedule": "* * * * *"}, auth)
    print("Testing one-minute scheduler override on the disposable instance.", flush=True)
    try:
        end = int(time.time() // 15) * 15
        samples = [metric_event(timestamp, 0) for timestamp in range(end - 360, end + 1, 15)]
        hec(local, state, samples)
        for _ in range(100):
            hec(local, state, [metric_event(time.time(), 0)])
            try:
                fired = local.request(
                    "/servicesNS/admin/openbao_observability/alerts/fired_alerts/" + identifier, auth=auth
                ).get("entry", [])
            except local.APIError as error:
                if error.status != 404:
                    raise
                fired = []
            if fired:
                print("Native scheduled trigger observed.", flush=True)
                break
            time.sleep(2)
        else:
            raise RuntimeError("Packaged alert did not create a native scheduled trigger")
        before = len(fired)
        # A future evaluation end after recovery excludes the old failing state.
        now = time.time()
        hec(local, state, [metric_event(now, 1)])
        for _ in range(100):
            hec(local, state, [metric_event(time.time(), 1)])
            jobs = local.request(endpoint + "/history?count=10", auth=auth).get("entry", [])
            recovered = []
            for job in jobs:
                if datetime.fromisoformat(job["published"].replace("Z", "+00:00")).timestamp() <= now + 30:
                    continue
                result = local.request("/services/search/jobs/" + urllib.parse.quote(job["name"], safe=""), auth=auth)[
                    "entry"
                ][0]["content"]
                if result.get("dispatchState") == "DONE" and int(result.get("resultCount", -1)) == 0:
                    recovered.append(job)
            if recovered:
                print("Native scheduled recovery observed.", flush=True)
                return {
                    "scheduled_trigger": True,
                    "scheduled_recovery_rows": 0,
                    "fired_records_before_recovery": before,
                    "outbound_actions": [],
                    "boundary": "Recovery stops matching results; no resolved notification is generated.",
                }
            time.sleep(2)
        raise RuntimeError("Recovered native condition still produced a firing result")
    finally:
        local.request(endpoint, {"disabled": True, "cron_schedule": "*/5 * * * *"}, auth)


def inventory(local):
    """Check expected-target absence only in a fresh, disposable installation."""
    lookup = "openbao_alert_targets.csv"
    if local.search("| inputlookup " + lookup):
        raise RuntimeError("Expected-target test requires an empty disposable lookup")
    coverage = json.loads((local.ROOT / "generated/splunk/alert-coverage.json").read_text())
    alert = next(item for item in coverage["alerts"] if item["id"] == "OpenBaoUnreachable")
    seed = (
        '| makeresults | eval cluster="compose", kubernetes_namespace="(none)", '
        'scrape_profile="all_nodes", instance="fixture" '
        "| table cluster kubernetes_namespace scrape_profile instance | outputlookup " + lookup
    )
    try:
        local.search(seed)
        if len(execute(local, alert, [])) != 1:
            raise RuntimeError("Missing expected target did not fire")
        if execute(local, alert, history(alert, False)):
            raise RuntimeError("Healthy expected target fired")
        local.search(seed.replace('cluster="compose"', 'cluster="other"'))
        if execute(local, alert, []):
            raise RuntimeError("Expected target from another deployment leaked into scope")
    finally:
        local.search("| inputlookup " + lookup + " | where 1=0 | outputlookup " + lookup)
    return {"checks": 3, "missing_target_fires": True, "healthy_target_recovers": True, "scope_isolated": True}


def metric_event(timestamp, value):
    return {
        "time": timestamp,
        "event": "metric",
        "index": "openbao_metrics",
        "host": "openbao-node0",
        "fields": {
            "cluster": "compose",
            "scrape_profile": "all_nodes",
            "instance": "scheduled:8200",
            "kubernetes_namespace": "(none)",
            "seal_state_source": "cluster",
            "job": "openbao",
            "metric_name:vault_core_unsealed": value,
        },
    }
