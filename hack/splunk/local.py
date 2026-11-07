#!/usr/bin/env python3
"""Manage a disposable local or SSH-hosted Splunk trial. Uses the Python standard library."""

import argparse
import configparser
import json
import math
import os
import re
import secrets
import ssl
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path

from support import write_private

ROOT = Path(__file__).resolve().parents[2]
DIRECTORY = ROOT / "examples/splunk"
TOOLS = Path(__file__).resolve().parent
API = "https://127.0.0.1:18089"
# This helper connects only to the local fixture API or a loopback SSH forward.
CONTEXT = ssl._create_unverified_context()


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise RuntimeError("Redirects are disabled for the loopback trial API")


class APIError(RuntimeError):
    def __init__(self, status, path):
        self.status = status
        # A server error body can echo submitted credentials or configuration.
        super().__init__(f"Splunk API returned HTTP {status} for {path}")


OPENER = urllib.request.build_opener(
    urllib.request.ProxyHandler({}), urllib.request.HTTPSHandler(context=CONTEXT), NoRedirect()
)


def environment():
    path = DIRECTORY / ".env"
    if not path.exists():
        raise RuntimeError("Run setup --accept-splunk-terms first")
    return dict(line.split("=", 1) for line in path.read_text().splitlines() if line and not line.startswith("#"))


def image(variable):
    """Read the Compose image pin instead of maintaining a second script pin."""
    sources = [DIRECTORY / "compose.yaml", ROOT / "examples/docker-compose/compose.yaml"]
    defaults = {
        match
        for source in sources
        for match in re.findall(r"\$\{" + re.escape(variable) + r":-([^}]+)\}", source.read_text())
    }
    if len(defaults) != 1:
        raise RuntimeError("Expected one Compose image default for " + variable)
    value = os.environ.get(variable) or environment().get(variable) or defaults.pop()
    registry = value.split("/", 1)[0]
    return value if "." in registry or ":" in registry or registry == "localhost" else "docker.io/" + value


def request(path, data=None, auth=None, method=None, timeout=45):
    payload = None if data is None else urllib.parse.urlencode(data, doseq=True).encode()
    headers = {} if auth is None else {"Authorization": auth}
    req = urllib.request.Request(
        API + path + ("&" if "?" in path else "?") + "output_mode=json", payload, headers, method=method
    )
    try:
        with OPENER.open(req, timeout=timeout) as response:
            content = response.read()
            return json.loads(content) if content else {}
    except urllib.error.HTTPError as exc:
        exc.close()
        raise APIError(exc.code, path) from None


def session(username="admin", password=None):
    result = request(
        "/services/auth/login",
        {"username": username, "password": environment()["SPLUNK_PASSWORD"] if password is None else password},
    )
    return "Splunk " + result["sessionKey"]


def delete(path, auth):
    request(path, auth=auth, method="DELETE")


def search(query, earliest="-15m", latest="now", auth=None, username="admin"):
    if not query.lstrip().startswith(("|", "search ")):
        query = "search " + query
    user_path = urllib.parse.quote(username, safe="")
    result = request(
        f"/servicesNS/{user_path}/openbao_observability/search/jobs",
        {
            "search": query,
            "exec_mode": "oneshot",
            "count": 1000,
            "namespace": "openbao_observability",
            "earliest_time": earliest,
            "latest_time": latest,
        },
        auth or session(),
    )
    for message in result.get("messages", []):
        if message.get("type") in ("WARN", "ERROR", "FATAL"):
            raise RuntimeError(f"Splunk search returned {message['type']}: {message.get('text', '')}")
    return result.get("results", [])


def setup(accept):
    if not accept:
        raise RuntimeError("Review the Splunk General Terms, then pass --accept-splunk-terms to initialize this trial")
    path = DIRECTORY / ".env"
    if not path.exists():
        with open(path, "x", encoding="utf-8", opener=lambda name, flags: os.open(name, flags, 0o600)) as stream:
            stream.write(
                "SPLUNK_PASSWORD="
                + secrets.token_urlsafe(30)
                + "\nSPLUNK_HEC_TOKEN="
                + str(uuid.uuid4())
                + "\nSPLUNK_GENERAL_TERMS=--accept-sgt-current-at-splunk-com\nSPLUNK_START_ARGS=--accept-license\n"
            )
    path.chmod(0o600)
    (DIRECTORY / ".runtime").mkdir(mode=0o700, exist_ok=True)
    print("Prepared ignored trial credentials; existing credentials and license data are preserved.")


def wait_ready(timeout):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            request(
                "/services/auth/login",
                {"username": "admin", "password": environment()["SPLUNK_PASSWORD"]},
                timeout=min(5, max(0.1, deadline - time.monotonic())),
            )
            print("Splunk trial API is ready.")
            return
        except (urllib.error.URLError, APIError):
            time.sleep(min(2, max(0, deadline - time.monotonic())))
    raise RuntimeError("Splunk trial API did not become ready before the deadline")


def status():
    auth = session()
    server = request("/services/server/info", auth=auth)["entry"][0]["content"]
    licenses = request("/services/licenser/licenses", auth=auth)["entry"]
    groups = request("/services/licenser/groups", auth=auth)["entry"]
    print(
        json.dumps(
            {
                "version": server.get("version"),
                "host": server.get("serverName"),
                "active_license_groups": [item["name"] for item in groups if item["content"].get("is_active")],
                "licenses": [
                    {key: item["content"].get(key) for key in ("label", "type", "expiration_time", "quota")}
                    for item in licenses
                ],
            },
            indent=2,
        )
    )


def audit():
    auth = session()
    env = os.environ | {"SPLUNK_URL": API, "SPLUNK_TOKEN": auth.removeprefix("Splunk "), "SPLUNK_AUTH_SCHEME": "Splunk"}
    tool = (
        [str(ROOT / "openbao-observability")]
        if (ROOT / "openbao-observability").exists()
        else ["go", "run", "./cmd/openbao-observability"]
    )
    subprocess.run(tool + ["validate", "splunk-queries", "--insecure-loopback"], cwd=ROOT, env=env, check=True)
    print("Synthetic audit queries passed.")


def install():
    app = ROOT / "generated/splunk/app/openbao_observability"
    auth = session()
    config = configparser.ConfigParser(interpolation=None)
    config.read(app / "default/macros.conf")
    prefix = "/servicesNS/nobody/openbao_observability"
    for name in config.sections():
        data = dict(config[name])
        path = prefix + "/configs/conf-macros/" + urllib.parse.quote(name, safe="")
        try:
            request(path, data, auth)
        except RuntimeError as error:
            if "HTTP 404" not in str(error):
                raise
            request(prefix + "/configs/conf-macros", data | {"name": name}, auth)
    for view in (app / "default/data/ui/views").glob("*.xml"):
        data = {"eai:data": view.read_text()}
        path = prefix + "/data/ui/views/" + view.stem
        try:
            request(path, data, auth)
        except RuntimeError as error:
            if "HTTP 404" not in str(error):
                raise
            request(prefix + "/data/ui/views", data | {"name": view.stem}, auth)
    for nav in (app / "default/data/ui/nav").glob("*.xml"):
        request(prefix + "/data/ui/nav/" + nav.stem, {"eai:data": nav.read_text()}, auth)
    print("Installed generated macros, navigation, and Dashboard Studio views through the API.")


def indexes():
    """Create only this disposable profile's named indexes; preserve existing data."""
    config = configparser.ConfigParser(interpolation=None)
    config.optionxform = str
    config.read(DIRECTORY / "local/indexes.conf")
    auth = session()
    for name in config.sections():
        path = "/services/data/indexes/" + name
        try:
            request(path, auth=auth)
        except RuntimeError as error:
            if "HTTP 404" not in str(error):
                raise
            keys = ("datatype", "maxTotalDataSizeMB", "frozenTimePeriodInSecs")
            data = {key: config[name][key] for key in keys if key in config[name]}
            request("/services/data/indexes", data | {"name": name}, auth)
    print("Verified disposable profile indexes; retained existing index data.")


def panels():
    report = []
    for path in sorted((ROOT / "generated/splunk/dashboards").glob("*.json")):
        dashboard = json.loads(path.read_text())
        tokens = {
            item["options"]["token"]: item["options"]["defaultValue"]
            for item in dashboard["inputs"].values()
            if item["type"] != "input.timerange"
        }
        for name, source in dashboard["dataSources"].items():
            query = source["options"]["query"]
            for token, value in tokens.items():
                query = query.replace("$" + token + "|s$", json.dumps(value))
            if re.search(r"\$[a-z_]+\|s\$", query):
                raise RuntimeError("Unresolved dashboard token in " + name)
            parameters = source["options"]["queryParameters"]
            earliest = parameters["earliest"]
            latest = parameters["latest"]
            rows = search(
                query, "-15m" if earliest.startswith("$") else earliest, "now" if latest.startswith("$") else latest
            )
            visualization = next(v for v in dashboard["visualizations"].values() if v["dataSources"]["primary"] == name)
            if rows and visualization["type"] == "splunk.line" and any("_time" not in row for row in rows):
                raise RuntimeError(f"{path.stem}/{name}: line chart lacks a time field")
            if rows and "seriesByName('value')" in visualization.get("options", {}).get("majorValue", ""):
                if any("value" not in row for row in rows):
                    raise RuntimeError(f"{path.stem}/{name}: metric stat lacks its value field")
            report.append({"dashboard": path.stem, "source": name, "rows": len(rows)})
            print(f"{path.stem}/{name}: {len(rows)} rows", flush=True)
    target = DIRECTORY / ".runtime"
    target.mkdir(mode=0o700, exist_ok=True)
    (target / "panels.json").write_text(json.dumps(report, indent=2) + "\n")


def collector(engine):
    runtime = DIRECTORY / ".runtime"
    runtime.mkdir(mode=0o700, exist_ok=True)
    for directory in ("completed", "platform"):
        (runtime / "logs" / directory).mkdir(parents=True, exist_ok=True, mode=0o700)
    (runtime / "archive").mkdir(exist_ok=True, mode=0o700)
    env_file = runtime / "collector.env"
    write_private(
        env_file,
        "SPLUNK_HEC_TOKEN="
        + environment()["SPLUNK_HEC_TOKEN"]
        + "\nSPLUNK_HEC_ENDPOINT=https://splunk:8088/services/collector\n",
    )
    name = "openbao-observability_splunk-otel_1" if engine == "podman" else "openbao-observability-splunk-otel-1"
    exists = (
        subprocess.run([engine, "inspect", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode == 0
    )
    if exists:
        subprocess.run([engine, "rm", "--force", name], check=True, stdout=subprocess.DEVNULL)
    command = [
        engine,
        "run",
        "--detach",
        "--name",
        name,
        "--user",
        "0",
        "--network",
        "openbao-observability_default",
        "--env-file",
        str(env_file),
        "--volume",
        str(DIRECTORY / "local/otel.yaml") + ":/etc/otel/config.yaml:ro",
        "--volume",
        "openbao-observability_splunk-otel-state:/var/lib/otelcol",
    ]
    command += ["--volume", str(runtime / "logs") + ":/var/log/qualification:ro"]
    command += ["--volume", str(runtime / "archive") + ":/var/lib/qualification/archive"]
    if engine == "podman":
        command += ["--signature-policy", str(DIRECTORY / "local/policy.json")]
    for node in range(3):
        for kind in ("audit", "logs"):
            command += [
                "--volume",
                f"openbao-observability_openbao-node{node}-{kind}:/var/log/openbao/node{node}/{kind}:ro",
            ]
    command += [
        image("SPLUNK_OTEL_IMAGE"),
        "--config=/etc/otel/config.yaml",
    ]
    subprocess.run(command, check=True, stdout=subprocess.DEVNULL)
    print("Started collector with preserved file offsets and current configuration.")


def synthetic_metrics():
    cases = json.loads((ROOT / "internal/splunk/testdata/metrics.json").read_text())
    report = []
    for case in cases:
        values, interval = case["values"], case["interval"]
        entries = ",".join(f"row={i + 1},{value}" for i, value in enumerate(values) if value is not None)
        query = (
            f"| makeresults count={len(values)} | streamstats count AS row "
            f"| eval _time=now()-{(len(values) - 1) * interval}+(row-1)*{interval}, "
            f'value=case({entries}), _timeseries="fixture", instance="fixture", cluster="synthetic", scrape_profile="all_nodes" '
            f"| where isnotnull(value) | `{case['macro']}` | stats latest(value) AS value"
        )
        rows = search(query)
        raw = rows[0].get("value") if rows else None
        actual = float(raw) if raw is not None else None
        expected = case["expected"]
        if (actual is None) != (expected is None) or (expected is not None and abs(actual - expected) > 0.00001):
            raise RuntimeError(f"{case['name']}: got {actual}, expected {expected}")
        report.append({"case": case["name"], "value": actual})
    print(json.dumps(report, indent=2))


def workload():
    # These credentials and HTTP endpoints belong only to the disposable Compose fixture.
    address = "http://127.0.0.1:18200/v1/"

    def bao(path, data, token=None):
        headers = {} if token is None else {"X-Vault-Token": token}
        body = None if data is None else json.dumps(data).encode()
        with urllib.request.urlopen(urllib.request.Request(address + path, body, headers), timeout=20) as response:
            raw = response.read()
            return json.loads(raw) if raw else {}

    for index in range(90):
        token = bao("auth/userpass/login/demo-admin", {"password": "openbao-observability"})["auth"]["client_token"]
        bao("secret/data/observability/splunk-parity", {"data": {"counter": index}}, token)
        bao("secret/data/observability/splunk-parity", None, token)
        bao("auth/token/create", {"ttl": "5s", "explicit_max_ttl": "5s", "display_name": "splunk-fixture"}, token)
        time.sleep(0.5)
    print("Completed 90 bounded fixture login/write/read/short-token cycles.")


def parity():
    # Use one evaluation end time and a shared five-minute window for both backends.
    end = int(time.time()) - 30
    queries = [
        ("active-nodes", "sum(vault_core_active)", 0),
        ("unsealed-nodes", "sum(openbao:core_unsealed:sum)", 0),
        ("autopilot-health", "max(vault_autopilot_healthy)", 0),
        ("autopilot-failure-tolerance", "max(vault_autopilot_failure_tolerance)", 0),
        ("request-rate", "sum(rate(vault_core_handle_request_count[5m]))", 0.1),
        (
            "request-latency",
            "sum(rate(vault_core_handle_request_sum[5m])) / sum(rate(vault_core_handle_request_count[5m]))",
            0.1,
        ),
        (
            "login-latency",
            "sum(rate(vault_core_handle_login_request_sum[5m])) / sum(rate(vault_core_handle_login_request_count[5m]))",
            0.1,
        ),
        (
            "token-check-latency",
            "sum(rate(vault_core_check_token_sum[5m])) / sum(rate(vault_core_check_token_count[5m]))",
            0.1,
        ),
    ]
    directory = ROOT / "generated/splunk/searches/openbao_overview"
    report = []
    for name, promql, tolerance in queries:
        query = (directory / (name + ".spl")).read_text()
        splunk = search(query, str(end - 300), str(end))
        url = "http://127.0.0.1:19090/api/v1/query?" + urllib.parse.urlencode({"query": promql, "time": end})
        with urllib.request.urlopen(url, timeout=20) as response:
            prom = json.load(response)["data"]["result"]
        actual = float(splunk[0]["value"]) if splunk else None
        expected = float(prom[0]["value"][1]) if prom else None
        if expected is not None and not math.isfinite(expected):
            expected = None
        if (actual is None) != (expected is None):
            raise RuntimeError(f"{name}: different data availability: Splunk={actual}, Prometheus={expected}")
        if expected is not None and abs(actual - expected) > max(1e-8, abs(expected) * tolerance):
            raise RuntimeError(f"{name}: Splunk={actual}, Prometheus={expected}, relative tolerance={tolerance}")
        report.append({"case": name, "splunk": actual, "prometheus": expected, "relative_tolerance": tolerance})
    target = DIRECTORY / ".runtime"
    target.mkdir(mode=0o700, exist_ok=True)
    (target / "parity.json").write_text(json.dumps({"end": end, "comparisons": report}, indent=2) + "\n")
    print(json.dumps(report, indent=2))


def synthetic_topology():
    def execute(rows, tail):
        data = json.dumps(json.dumps(rows, separators=(",", ":")))
        source = (
            f"| makeresults | eval payload={data} | spath input=payload path={{}} output=row "
            "| mvexpand row | spath input=row | eval _time=now()+offset,bucket_time=_time, "
            '_timeseries=instance."/".metric_name,cluster="synthetic",scrape_profile="all_nodes" '
            "| fields - row payload offset"
        )
        return search(source + "\n" + tail)

    def row(instance, metric, value, offset, **extra):
        return {"instance": instance, "metric_name": metric, "value": value, "offset": offset} | extra

    queue = "vault_expire_job_manager_total_jobs"
    active = "vault_core_active"
    report = []
    for name, rows, expected in [
        ("inactive-stale-queue", [row("a", active, 0, -15), row("a", queue, 999, -15)], None),
        ("active-zero-queue", [row("a", active, 1, -15), row("a", queue, 0, -15)], 0),
        ("missing-queue", [row("a", active, 1, -15)], None),
    ]:
        result = execute(rows, "| `openbao_metric_active_expiration` | stats latest(value) AS value")
        actual = float(result[0]["value"]) if result and "value" in result[0] else None
        if actual != expected:
            raise RuntimeError(f"{name}: got {actual}, expected {expected}")
        report.append({"case": name, "value": actual})
    transition = [
        row("a", active, 1, -30),
        row("a", queue, 100, -30),
        row("b", active, 0, -30),
        row("b", queue, 999, -30),
        row("a", active, 0, -15),
        row("a", queue, 100, -15),
        row("b", active, 1, -15),
        row("b", queue, 50, -15),
    ]
    result = execute(
        transition, "| `openbao_metric_active_expiration` | `openbao_metric_slope5m` | stats latest(value) AS value"
    )
    if result and "value" in result[0]:
        raise RuntimeError("leader transition joined independent target histories")
    report.append({"case": "leader-transition-independent-histories", "value": None})
    directory = ROOT / "generated/splunk/searches/openbao_overview"

    def panel_tail(name):
        query = (directory / (name + ".spl")).read_text()
        return re.sub(r"^\| `openbao_metric_samples\([^`]*\)`", "", query, count=1)

    rows = []
    for step in range(21):
        for instance, count, total in [("a", step * 2, step * 20), ("b", step, step * 100)]:
            rows += [
                row(instance, "vault_core_handle_login_request_count", count, (step - 20) * 15),
                row(instance, "vault_core_handle_login_request_sum", total, (step - 20) * 15),
            ]
    result = execute(rows, panel_tail("login-latency"))
    actual = float(result[0]["value"])
    if abs(actual - 40) > 0.00001:
        raise RuntimeError(f"weighted latency: got {actual}, expected 40 ms")
    report.append({"case": "weighted-latency-ms", "value": actual})
    for entry in rows:
        entry["value"] = 100 if entry["metric_name"].endswith("_sum") else 10
    if execute(rows, panel_tail("login-latency")):
        raise RuntimeError("idle latency returned a numeric value")
    report.append({"case": "idle-latency-is-absent", "value": None})
    for name, rows, expected in [
        (
            "prefer-cluster-seal-state",
            [
                row("a", "vault_core_unsealed", 1, -15, seal_state_source="startup"),
                row("a", "vault_core_unsealed", 0, -15, seal_state_source="cluster"),
            ],
            0,
        ),
        ("startup-seal-state-fallback", [row("a", "vault_core_unsealed", 1, -15, seal_state_source="startup")], 1),
    ]:
        result = execute(rows, panel_tail("unsealed-nodes"))
        actual = float(result[0]["value"])
        if actual != expected:
            raise RuntimeError(f"{name}: got {actual}, expected {expected}")
        report.append({"case": name, "value": actual})
    print(json.dumps(report, indent=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    configure = commands.add_parser("setup")
    configure.add_argument("--accept-splunk-terms", action="store_true")
    commands.add_parser("status")
    show_image = commands.add_parser("image")
    show_image.add_argument(
        "variable", choices=("SPLUNK_IMAGE", "SPLUNK_OTEL_IMAGE", "SPLUNK_BLACKBOX_IMAGE", "OPENBAO_IMAGE")
    )
    wait = commands.add_parser("wait")
    wait.add_argument("--timeout", type=int, default=300)
    commands.add_parser("audit")
    commands.add_parser("install")
    commands.add_parser("indexes")
    access = commands.add_parser("access")
    access.add_argument("--keep-for-browser", action="store_true")
    commands.add_parser("access-cleanup")
    commands.add_parser("panels")
    commands.add_parser("synthetic-metrics")
    commands.add_parser("workload")
    commands.add_parser("parity")
    commands.add_parser("synthetic-topology")
    commands.add_parser("synthetic-families")
    collect = commands.add_parser("collector")
    collect.add_argument("--engine", choices=("docker", "podman"), default="docker")
    logs = commands.add_parser("log-delivery")
    logs.add_argument("--engine", choices=("docker", "podman"), default="docker")
    restart = commands.add_parser("offset-restart")
    restart.add_argument("--engine", choices=("docker", "podman"), default="docker")
    external = commands.add_parser("external")
    external.add_argument("--engine", choices=("docker", "podman"), default="docker")
    stop = commands.add_parser("external-stop")
    stop.add_argument("--engine", choices=("docker", "podman"), default="docker")
    commands.add_parser("verify-external")
    commands.add_parser("history")
    query = commands.add_parser("search")
    query.add_argument("query")
    args = parser.parse_args()
    if args.command == "setup":
        setup(args.accept_splunk_terms)
    elif args.command == "status":
        status()
    elif args.command == "image":
        print(image(args.variable))
    elif args.command == "wait":
        if args.timeout <= 0:
            raise RuntimeError("--timeout must be positive")
        wait_ready(args.timeout)
    elif args.command == "audit":
        audit()
    elif args.command == "install":
        install()
    elif args.command == "indexes":
        indexes()
    elif args.command in ("access", "access-cleanup"):
        import access_checks

        if args.command == "access":
            access_checks.run(sys.modules[__name__], args.keep_for_browser)
        else:
            access_checks.cleanup(sys.modules[__name__])
    elif args.command == "panels":
        panels()
    elif args.command == "synthetic-topology":
        synthetic_topology()
    elif args.command == "synthetic-families":
        from family_checks import run

        run(ROOT, search)
    elif args.command == "parity":
        parity()
    elif args.command == "workload":
        workload()
    elif args.command == "synthetic-metrics":
        synthetic_metrics()
    elif args.command == "collector":
        collector(args.engine)
    elif args.command == "log-delivery":
        import log_checks

        log_checks.run(sys.modules[__name__], args.engine)
    elif args.command == "offset-restart":
        import log_checks

        log_checks.restart(sys.modules[__name__], args.engine)
    elif args.command == "external":
        import external_checks

        external_checks.start(sys.modules[__name__], args.engine)
    elif args.command == "external-stop":
        import external_checks

        external_checks.stop(sys.modules[__name__], args.engine)
    elif args.command == "verify-external":
        import external_checks

        external_checks.verify(sys.modules[__name__])
    elif args.command == "history":
        import history_checks

        history_checks.run(sys.modules[__name__])
    elif args.command == "search":
        print(json.dumps(search(args.query), indent=2))


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, urllib.error.URLError, subprocess.CalledProcessError) as error:
        sys.exit(str(error))
