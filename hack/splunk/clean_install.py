"""Install only the release archive into a fresh, disposable Splunk instance."""

import configparser
import hashlib
import json
import secrets
import subprocess
import tarfile
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
import xml.etree.ElementTree as ET
from contextlib import contextmanager

from support import write_private

APP = "/servicesNS/nobody/openbao_observability"


@contextmanager
def client(local, state):
    api, environment = local.API, local.environment
    local.API = state["api"]
    local.environment = lambda: state["environment"]
    try:
        yield
    finally:
        local.API, local.environment = api, environment


def command(engine, *args, **kwargs):
    return subprocess.run([engine, *args], check=True, **kwargs)


def run(local, engine, package, keep=False):
    if not package.is_file():
        raise RuntimeError("Build the installable .spl package first")
    run_id = uuid.uuid4().hex[:10]
    name = "openbao-splunk-clean-" + run_id
    network = name + "-network"
    directory = local.DIRECTORY / ".runtime" / name
    directory.mkdir(parents=True, mode=0o700)
    accepted = local.environment()["SPLUNK_GENERAL_TERMS"]
    environment = {
        "SPLUNK_PASSWORD": secrets.token_urlsafe(32),
        "SPLUNK_HEC_TOKEN": str(uuid.uuid4()),
        "SPLUNK_GENERAL_TERMS": accepted,
        "SPLUNK_START_ARGS": "--accept-license",
    }
    env_file = directory / "credentials.env"
    write_private(env_file, "".join(key + "=" + value + "\n" for key, value in environment.items()))
    image = local.image("SPLUNK_IMAGE")
    policy = ["--signature-policy", str(local.DIRECTORY / "local/policy.json")] if engine == "podman" else []
    report = None
    try:
        command(engine, "network", "create", network, stdout=subprocess.DEVNULL)
        command(
            engine,
            "run",
            *policy,
            "--detach",
            "--name",
            name,
            "--hostname",
            name,
            "--user",
            "0",
            "--network",
            network,
            "--env-file",
            str(env_file),
            "--publish",
            "127.0.0.1::8089",
            "--publish",
            "127.0.0.1::8088",
            image,
            stdout=subprocess.DEVNULL,
        )
        ports = {}
        for port in (8089, 8088):
            address = command(engine, "port", name, str(port) + "/tcp", capture_output=True, text=True).stdout.strip()
            if not address.startswith("127.0.0.1:"):
                raise RuntimeError("Clean qualification ports must remain on loopback")
            ports[port] = address
        state = {
            "api": "https://" + ports[8089],
            "hec": "https://" + ports[8088],
            "environment": environment,
            "container": name,
            "network": network,
        }
        write_private(directory / "private.json", json.dumps(state))
        with client(local, state):
            local.wait_ready(420)
            report = install(local, engine, name, package)
            wait_hec(local, state)
            report["representative_data"] = ingest_control(local, state)
            local.panels()
            report["panel_queries"] = 289
            import alert_checks

            report["native_alerts"] = alert_checks.run(local)
            report["expected_targets"] = alert_checks.inventory(local)
            report["scheduled_alert"] = alert_checks.scheduled(local, state)
            report["package_sha256"] = hashlib.sha256(package.read_bytes()).hexdigest()
            report["retained"] = keep
    except BaseException:
        logs = subprocess.run([engine, "logs", name], capture_output=True, text=True)
        write_private(directory / "startup.log", logs.stdout + logs.stderr)
        raise
    finally:
        if not keep:
            cleanup(engine, name, network, directory)
    if report is not None:
        report["cleanup_completed"] = not keep
        write_private(directory / "report.json", json.dumps(report, indent=2) + "\n")
        print(json.dumps(report, indent=2), flush=True)


def cleanup(engine, name, network, directory):
    failed = False
    for arguments in [("rm", "--force", "--volumes", name), ("network", "rm", network)]:
        result = subprocess.run([engine, *arguments], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        failed = failed or result.returncode != 0
    for arguments in [("container", "inspect", name), ("network", "inspect", network)]:
        result = subprocess.run([engine, *arguments], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        failed = failed or result.returncode == 0
    if failed:
        raise RuntimeError(
            "Disposable resource cleanup failed; private recovery files are retained in " + str(directory)
        )
    (directory / "credentials.env").unlink(missing_ok=True)
    (directory / "private.json").unlink(missing_ok=True)


def install(local, engine, name, package):
    auth = local.session()
    try:
        local.request("/services/apps/local/openbao_observability", auth=auth)
    except local.APIError as error:
        if error.status != 404:
            raise
    else:
        raise RuntimeError("Clean test already has an OpenBao app; refusing to test an upgrade as an installation")
    with tarfile.open(package) as archive:
        conf = configparser.ConfigParser(interpolation=None)
        conf.read_string(archive.extractfile("openbao_observability/default/app.conf").read().decode())
        version = conf["launcher"]["version"]
    command(engine, "cp", str(package), name + ":/tmp/openbao-clean.spl", stdout=subprocess.DEVNULL)
    command(engine, "exec", name, "chmod", "0644", "/tmp/openbao-clean.spl", stdout=subprocess.DEVNULL)
    local.request(
        "/services/apps/local",
        {"name": "/tmp/openbao-clean.spl", "filename": True, "update": False},
        auth=auth,
        timeout=120,
    )
    installed = local.request("/services/apps/local/openbao_observability", auth=auth)["entry"][0]
    if installed["content"]["version"] != version:
        raise RuntimeError("Installed app version differs from the release artifact")
    local.indexes()
    views, panels = verify_objects(local, auth)
    searches = local.request(APP + "/saved/searches?count=0", auth=auth)["entry"]
    native = [entry for entry in searches if entry["name"].startswith("OpenBao")]
    manifest = json.loads((local.ROOT / "generated/splunk/alert-coverage.json").read_text())
    expected = {item["id"] for item in manifest["alerts"] if item["status"] == "mapped"}
    if {entry["name"] for entry in native} != expected:
        raise RuntimeError("Installed alert inventory differs from the release profile")
    for entry in native:
        content = entry["content"]
        if not content["disabled"] or content.get("actions"):
            raise RuntimeError("Packaged native alerts must be disabled with no outbound actions")
    return {
        "fresh_app_absent_before_install": True,
        "archive_only_install": True,
        "version": version,
        "views": views,
        "panels": panels,
        "native_alerts_disabled": len(native),
        "app_acl": installed["acl"]["perms"],
    }


def verify_objects(local, auth):
    views, panels = 0, 0
    directory = local.ROOT / "generated/splunk/app/openbao_observability"
    for path in (directory / "default/data/ui/views").glob("*.xml"):
        entry = local.request(APP + "/data/ui/views/" + path.stem, auth=auth)["entry"][0]
        document = json.loads(ET.fromstring(entry["content"]["eai:data"]).findtext("definition"))
        panels += len(document["visualizations"])
        views += 1
    if views != 15 or panels != 289:
        raise RuntimeError(f"Installed package contains {views} views / {panels} panels")
    config = configparser.ConfigParser(interpolation=None)
    config.read(directory / "default/macros.conf")
    for macro in config.sections():
        local.request(APP + "/configs/conf-macros/" + urllib.parse.quote(macro, safe=""), auth=auth)
    local.request(APP + "/data/ui/nav/default", auth=auth)
    return views, panels


def hec(local, state, events):
    request = urllib.request.Request(
        state["hec"] + "/services/collector/event",
        "\n".join(json.dumps(event) for event in events).encode(),
        {"Authorization": "Splunk " + state["environment"]["SPLUNK_HEC_TOKEN"]},
    )
    with local.OPENER.open(request, timeout=20) as response:
        if json.load(response).get("code") != 0:
            raise RuntimeError("Clean test HEC ingestion failed")


def wait_hec(local, state):
    for _ in range(60):
        request = urllib.request.Request(
            state["hec"] + "/services/collector/health",
            headers={"Authorization": "Splunk " + state["environment"]["SPLUNK_HEC_TOKEN"]},
        )
        try:
            with local.OPENER.open(request, timeout=3) as response:
                if response.status == 200:
                    return
        except (urllib.error.URLError, OSError):
            pass
        time.sleep(1)
    raise RuntimeError("Clean test HEC endpoint did not become ready")


def ingest_control(local, state):
    now = time.time()
    labels = {
        "cluster": "compose",
        "scrape_profile": "all_nodes",
        "instance": "clean:8200",
        "kubernetes_namespace": "(none)",
        "job": "openbao",
    }
    hec(
        local,
        state,
        [
            {
                "time": now,
                "event": "metric",
                "index": "openbao_metrics",
                "host": "openbao-node0",
                "fields": labels | {"metric_name:vault_core_active": 1, "metric_name:vault_core_unsealed": 1},
            },
            {
                "time": now,
                "index": "openbao_audit",
                "sourcetype": "openbao:audit",
                "host": "openbao-node0",
                "event": json.dumps(
                    {
                        "type": "request",
                        "request": {"id": "clean-install-control", "path": "sys/health", "operation": "read"},
                    }
                ),
            },
        ],
    )
    for _ in range(60):
        metric = local.search('| `openbao_metric_samples("vault_core_active")` | stats latest(value) AS value')
        audit = local.search(
            '`openbao_audit_events` | search request_id="clean-install-control" | stats count AS count'
        )
        if metric and metric[0].get("value") == "1" and audit and int(audit[0]["count"]) == 1:
            return {"active_nodes": 1, "audit_events": 1}
        time.sleep(1)
    raise RuntimeError("Clean package did not query its representative metric/audit controls")
