"""Test long-window history guards with in-memory backfill; do not index samples."""

import json
import math

from family_checks import substitute


def run(local):
    dashboard = json.loads((local.ROOT / "generated/splunk/dashboards/openbao_slo_availability.json").read_text())
    cases = [
        ("complete history", "availability-30d", 172800, 15, 1, 0, 1),
        ("target-weighted complete history", "availability-30d", 172800, 15, 2, 0, 0.5),
        ("exhausted full-window budget", "error-budget-remaining", 172800, 15, 2, 0, -49),
        ("short history", "availability-30d", 100, 15, 1, 0, None),
        ("sparse full-span history", "availability-30d", 1729, 1500, 1, 0, None),
        ("stale complete history", "availability-30d", 172800, 15, 1, 120, None),
    ]
    results = []
    for name, panel, samples, interval, targets, stale, expected in cases:
        query = dashboard["dataSources"]["ds_" + panel.replace("-", "_")]["options"]["query"]
        source = (
            f"| makeresults count={(samples * targets + 999) // 1000} | streamstats count AS chunk "
            f"| eval item=mvrange(0,1000) | mvexpand item "
            f"| eval row=(chunk-1)*1000+item+1 | where row<={samples * targets} | addinfo "
            f'| eval position=(row-1)-floor((row-1)/{samples})*{samples}, target=if(row<={samples},"a","b"), '
            f"_time=info_max_time-{stale}-{(samples - 1) * interval}+position*{interval}, "
            'bucket_time=_time, _timeseries=target, value=if(target="a",1,0), '
            'metric_name="probe_success", cluster="compose", scrape_profile="external", '
            'instance=target, job="openbao-health", kubernetes_namespace="(none)", openbao_namespace="(root)"'
        )
        query = source + query[query.index("\n") :]
        query = query.replace("$slo_target|s$", '"0.99"')
        rows = local.search(substitute(query, dashboard), earliest="-30d")
        actual = rows[0].get("value") if rows else None
        if (actual is None) != (expected is None) or (
            expected is not None and not math.isclose(float(actual), expected, abs_tol=0.00001)
        ):
            raise RuntimeError(f"{name}: value={actual}, expected={expected}")
        results.append({"case": name, "value": actual})
        print(name + ": passed", flush=True)
    (local.DIRECTORY / ".runtime/history.json").write_text(json.dumps(results, indent=2) + "\n")
    print("Passed six history checks. Backfill is generated in memory; no 30-day collection claim.")
