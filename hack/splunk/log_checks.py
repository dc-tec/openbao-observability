"""Check completed-request collection and recorded Kubernetes Event ingestion."""

import json
import subprocess
import time
import uuid
from datetime import datetime, timezone


def run(local, engine):
    directory = local.DIRECTORY / ".runtime/logs"
    completed = directory / "completed"
    platform = directory / "platform"
    for path in (completed, platform):
        path.mkdir(parents=True, exist_ok=True, mode=0o700)
    run_id = uuid.uuid4().hex
    started = time.time()
    config = local.DIRECTORY / ".runtime/completed.hcl"
    config.write_text(
        'disable_mlock = true\nlog_level = "info"\nlog_format = "json"\n'
        'log_requests_level = "info"\nlog_file = "/qualification/logs/' + run_id + '.log"\n'
    )
    config.chmod(0o600)
    name = "openbao-splunk-completed-" + run_id[:8]
    command = [
        engine,
        "run",
        "--detach",
        "--name",
        name,
        "--user",
        "0",
        "--network",
        "none",
        "--entrypoint",
        "/usr/bin/bao",
        "--volume",
        str(config) + ":/qualification/completed.hcl:ro",
        "--volume",
        str(completed) + ":/qualification/logs",
    ]
    if engine == "podman":
        command += ["--signature-policy", str(local.DIRECTORY / "local/policy.json")]
    command += [
        local.image("OPENBAO_IMAGE"),
        "server",
        "-dev",
        "-dev-root-token-id=qualification-only",
        "-dev-listen-address=127.0.0.1:8200",
        "-config=/qualification/completed.hcl",
    ]
    subprocess.run(command, check=True, stdout=subprocess.DEVNULL)
    try:
        ready = False
        for _ in range(30):
            check = subprocess.run(
                [engine, "exec", name, "bao", "status", "-address=http://127.0.0.1:8200"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            if check.returncode == 0:
                ready = True
                break
            time.sleep(1)
        if not ready:
            log = subprocess.run([engine, "logs", name], capture_output=True)
            target = local.DIRECTORY / ".runtime/completed-startup.log"
            target.write_bytes(log.stdout + log.stderr)
            target.chmod(0o600)
            raise RuntimeError("Isolated completed-request fixture did not become ready; inspect private startup log")
        for i in range(3):
            subprocess.run(
                [
                    engine,
                    "exec",
                    "--env",
                    "BAO_TOKEN=qualification-only",
                    name,
                    "bao",
                    "kv",
                    "put",
                    "-address=http://127.0.0.1:8200",
                    f"secret/qualification-{run_id}-{i}",
                    "value=synthetic",
                ],
                check=True,
                stdout=subprocess.DEVNULL,
            )
    finally:
        subprocess.run([engine, "rm", "--force", name], check=True, stdout=subprocess.DEVNULL)
    expected = 0
    for path in completed.glob(run_id + "*"):
        for line in path.read_text().splitlines():
            try:
                expected += json.loads(line).get("@message") == "completed_request"
            except ValueError:
                continue
    if expected < 3:
        raise RuntimeError("OpenBao emitted no completed-request logging control records")
    now = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    records = []
    for version, reason in [("v1", "FailedMount"), ("events.k8s.io/v1", "Unhealthy")]:
        event = {
            "apiVersion": version,
            "kind": "Event",
            "eventTime": now,
            "type": "Warning",
            "reason": reason,
            "metadata": {"namespace": "qualification", "name": "fixture-" + run_id},
            "involvedObject" if version == "v1" else "regarding": {"kind": "Pod", "name": "openbao-qualification"},
            "message" if version == "v1" else "note": "Recorded test event; no Kubernetes cluster is queried.",
        }
        records.append({"cluster": "compose", "object": event})
    (platform / (run_id + ".jsonl")).write_text("".join(json.dumps(record) + "\n" for record in records))
    query = '`openbao_completed_events` | search host="qualification-completed" | stats count AS count'
    completed_count, platform_count = 0, 0
    for _ in range(60):
        completed_count = int(local.search(query, earliest=str(started - 1))[0]["count"])
        platform_rows = local.search(
            '`openbao_platform_events` | search host="qualification-platform"'
            ' pod="openbao-qualification" | stats count AS count',
            earliest=str(started - 1),
        )
        platform_count = int(platform_rows[0]["count"])
        if completed_count == expected and platform_count == 2:
            break
        time.sleep(1)
    if completed_count != expected or platform_count != 2:
        raise RuntimeError(f"Log delivery counts: completed={completed_count}/{expected}, platform={platform_count}/2")
    escaped = local.search(
        'index=openbao_operational host="qualification-completed" | stats count AS count', earliest=str(started - 1)
    )
    if int(escaped[0]["count"]) != 0:
        raise RuntimeError("Completed-request records leaked into the operational index")
    result = {
        "completed": {"emitted": expected, "indexed": completed_count, "isolated_server_removed": True},
        "platform": {"indexed": platform_count, "source": "Recorded Event JSON, not a live Kubernetes cluster"},
        "operational_index_isolation": True,
    }
    target = local.DIRECTORY / ".runtime/log-delivery.json"
    target.write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))


def restart(local, engine):
    queries = {
        "completed": '`openbao_completed_events` | search host="qualification-completed" | stats count AS count',
        "platform": '`openbao_platform_events` | search host="qualification-platform" | stats count AS count',
    }
    before = {name: int(local.search(query, earliest="-24h")[0]["count"]) for name, query in queries.items()}
    if not all(before.values()):
        raise RuntimeError("Run log-delivery before checking persisted receiver offsets")
    local.collector(engine)
    # Allow receivers to discover the same existing files after replacement.
    time.sleep(8)
    after = {name: int(local.search(query, earliest="-24h")[0]["count"]) for name, query in queries.items()}
    if before != after:
        raise RuntimeError("Receiver replacement changed controlled log counts: " + str((before, after)))
    result = {
        "before": before,
        "after": after,
        "boundary": "One controlled restart with acknowledged records; not an exactly-once delivery guarantee.",
    }
    (local.DIRECTORY / ".runtime/offset-restart.json").write_text(json.dumps(result, indent=2) + "\n")
    print(json.dumps(result, indent=2))
