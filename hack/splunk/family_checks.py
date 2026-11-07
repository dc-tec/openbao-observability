"""Run bounded semantic fixtures through installed macros and generated panel searches."""

import configparser
import json
import math
from pathlib import Path


def fixture_search(events):
    """Construct in-memory rows. No test events are indexed."""
    fields = sorted({key for event in events for key in event})
    query = f"| makeresults count={max(len(events), 1)} | streamstats count AS fixture_row"
    if not events:
        return query + " | where 1=0"
    for field in fields:
        entries = ",".join(
            f"fixture_row={i + 1},{json.dumps(event[field])}" for i, event in enumerate(events) if field in event
        )
        query += f" | eval {field}=case({entries})"
    return query + " | eval _time=now()+coalesce(time_offset,0), bucket_time=_time"


def substitute(query, dashboard):
    for item in dashboard["inputs"].values():
        options = item["options"]
        if item["type"] != "input.timerange":
            query = query.replace("$" + options["token"] + "|s$", json.dumps(options["defaultValue"]))
    return query


def metric_events(series):
    events = []
    for item in series:
        labels = {
            "cluster": "compose",
            "scrape_profile": "all_nodes",
            "instance": "fixture",
            "kubernetes_namespace": "(none)",
            "openbao_namespace": "(root)",
        } | item["labels"]
        values = item["values"]
        interval = item.get("interval", 15)
        for i, value in enumerate(values):
            if value is not None:
                events.append(
                    labels
                    | {
                        "_timeseries": json.dumps(labels, sort_keys=True),
                        "value": value,
                        "time_offset": -(len(values) - 1 - i) * interval,
                    }
                )
    return events


def check_result(case, rows):
    serialized = json.dumps(rows)
    if "FIXTURE_SECRET" in serialized or '"_raw"' in serialized:
        raise RuntimeError(case["name"] + ": leaked raw or secret fixture fields")
    if "expectedRows" in case and len(rows) != case["expectedRows"]:
        raise RuntimeError(f"{case['name']}: {len(rows)} rows, expected {case['expectedRows']}")
    if "expectedValue" in case:
        actual = rows[0].get("value") if rows else None
        expected = case["expectedValue"]
        if (actual is None) != (expected is None) or (
            expected is not None and not math.isclose(float(actual), expected, abs_tol=0.00001)
        ):
            raise RuntimeError(f"{case['name']}: got {actual}, expected {expected}")
    if "expectedLastValues" in case:
        last = rows[-1] if rows else {}
        values = sorted(float(v) for k, v in last.items() if not k.startswith("_"))
        if len(values) != len(case["expectedLastValues"]) or any(
            not math.isclose(a, b, abs_tol=0.00001) for a, b in zip(values, case["expectedLastValues"])
        ):
            raise RuntimeError(f"{case['name']}: got {values}, expected {case['expectedLastValues']}")


def run(root, search):
    root = Path(root)
    config = configparser.ConfigParser(interpolation=None)
    config.read(root / "generated/splunk/app/openbao_observability/default/macros.conf")
    cases = json.loads((root / "internal/splunk/testdata/families.json").read_text())
    report = []
    for case in cases:
        path = root / "generated/splunk/dashboards" / (case["dashboard"] + ".json")
        dashboard = json.loads(path.read_text())
        source = dashboard["dataSources"]["ds_" + case["panel"].replace("-", "_")]
        query = source["options"]["query"]
        if case["source"] == "metrics":
            events = metric_events(case["series"])
            query = fixture_search(events) + query[query.index("\n") :]
        else:
            events = [
                {"_raw": json.dumps(event), "host": "fixture-node", "time_offset": -60} for event in case["events"]
            ]
            macro = "openbao_" + case["source"] + "_events"
            definition = config[macro]["definition"].split("`openbao_" + case["source"] + "_source`", 1)[1]
            query = query.replace("`" + macro + "`", fixture_search(events) + definition, 1)
        for token, value in case.get("tokens", {}).items():
            query = query.replace("$" + token + "|s$", json.dumps(value))
        query = substitute(query, dashboard)
        rows = search(query, earliest="-30d")
        check_result(case, rows)
        report.append({"case": case["name"], "rows": len(rows)})
        print(case["name"] + ": passed", flush=True)
    target = root / "examples/splunk/.runtime/families.json"
    target.parent.mkdir(mode=0o700, exist_ok=True)
    target.write_text(json.dumps(report, indent=2) + "\n")
    print(f"Passed {len(cases)} native family fixtures; no events were indexed.")
