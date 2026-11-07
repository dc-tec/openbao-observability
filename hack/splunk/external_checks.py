"""Start real health probes and the trial local-file archive health exporter."""

import json
import os
import signal
import subprocess
import sys
import time


def start(local, engine):
    runtime = local.DIRECTORY / ".runtime"
    (runtime / "archive").mkdir(parents=True, exist_ok=True, mode=0o700)
    import archive_status

    archive_status.observe(runtime / "archive")
    policy = ["--signature-policy", str(local.DIRECTORY / "local/policy.json")] if engine == "podman" else []
    binary = runtime / "bin/archive-health"
    if engine == "podman":
        if not binary.exists():
            raise RuntimeError(
                "Build and copy the Linux archive-health binary as documented before external collection"
            )
        subprocess.run(
            [
                engine,
                "build",
                *policy,
                # Scratch plus a repository-built binary needs no cached base layers.
                "--no-cache",
                "--layers=false",
                "--pull=never",
                "--file",
                str(local.DIRECTORY / "local/archive.Containerfile"),
                "--tag",
                "localhost/openbao-splunk-archive:qualification",
                str(binary.parent),
            ],
            check=True,
            stdout=subprocess.DEVNULL,
        )
    else:
        subprocess.run(
            [
                engine,
                "build",
                "--file",
                str(local.ROOT / "examples/audit-archive-health/exporter/Dockerfile"),
                "--tag",
                "localhost/openbao-splunk-archive:qualification",
                str(local.ROOT),
            ],
            check=True,
        )
    for name, alias, image, arguments, volumes in [
        (
            "openbao-splunk-blackbox",
            "splunk-blackbox",
            local.image("SPLUNK_BLACKBOX_IMAGE"),
            ["--config.file=/config/blackbox.yaml"],
            [str(local.DIRECTORY / "local/blackbox.yaml") + ":/config/blackbox.yaml:ro"],
        ),
        (
            "openbao-splunk-archive",
            "splunk-archive-health",
            "localhost/openbao-splunk-archive:qualification",
            ["--enabled", "--status-file=/archive/status.json", "--listen-address=:19110"],
            [str(runtime / "archive") + ":/archive:ro"],
        ),
    ]:
        if (
            subprocess.run([engine, "inspect", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode
            == 0
        ):
            subprocess.run([engine, "rm", "--force", name], check=True, stdout=subprocess.DEVNULL)
        command = [
            engine,
            "run",
            *policy,
            "--detach",
            "--name",
            name,
            "--user",
            "0",
            "--network",
            "openbao-observability_default",
            "--network-alias",
            alias,
        ]
        for volume in volumes:
            command += ["--volume", volume]
        subprocess.run(command + [image] + arguments, check=True, stdout=subprocess.DEVNULL)
    if engine == "podman":
        subprocess.run(
            ["systemctl", "--user", "stop", "openbao-splunk-archive-status"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        subprocess.run(
            [
                "systemd-run",
                "--user",
                "--unit=openbao-splunk-archive-status",
                "--collect",
                sys.executable,
                str(local.TOOLS / "archive_status.py"),
                "--loop",
            ],
            check=True,
        )
    else:
        stop_observer(local)
        process = subprocess.Popen(
            [sys.executable, str(local.TOOLS / "archive_status.py"), "--loop"],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            start_new_session=True,
        )
        path = runtime / "archive-status.pid"
        path.write_text(str(process.pid))
        path.chmod(0o600)
    print("Started internal health probe and local audit-file health exporter; no new host ports.")


def verify(local):
    checks = {"probe": False, "negative_probe": False, "archive": False}
    for _ in range(60):
        rows = local.search(
            '| `openbao_metric_samples("probe_success")` | `openbao_metric_fresh`'
            ' | search cluster="compose" job="openbao-health" | stats latest(value) AS value'
        )
        checks["probe"] = bool(rows and float(rows[0].get("value", -1)) == 1)
        rows = local.search(
            '| `openbao_metric_samples("probe_success")` | `openbao_metric_fresh`'
            ' | search job="qualification-negative" | stats latest(value) AS value'
        )
        checks["negative_probe"] = bool(rows and float(rows[0].get("value", -1)) == 0)
        rows = local.search(
            '| `openbao_metric_samples("openbao_audit_archive_delivery_success")`'
            ' | `openbao_metric_fresh` | search backend="qualification-local-file"'
            " | stats latest(value) AS value"
        )
        checks["archive"] = bool(rows and float(rows[0].get("value", -1)) == 1)
        if all(checks.values()):
            break
        time.sleep(1)
    if not all(checks.values()):
        raise RuntimeError("External collection checks did not receive healthy fresh samples: " + str(checks))
    report = {
        "health_probe_received": True,
        "failed_probe_received_as_zero": True,
        "local_archive_readback_received": True,
        "boundary": "A local fixture file sink; not remote object storage, immutable retention, or Kubernetes collection.",
    }
    (local.DIRECTORY / ".runtime/external.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


def stop_observer(local):
    path = local.DIRECTORY / ".runtime/archive-status.pid"
    if not path.exists():
        return
    pid = int(path.read_text())
    command = subprocess.run(["ps", "-p", str(pid), "-o", "command="], capture_output=True, text=True).stdout
    if str(local.TOOLS / "archive_status.py") in command:
        os.kill(pid, signal.SIGTERM)
    path.unlink()


def stop(local, engine):
    for name in ("openbao-splunk-blackbox", "openbao-splunk-archive"):
        if (
            subprocess.run([engine, "inspect", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL).returncode
            == 0
        ):
            subprocess.run([engine, "stop", name], check=True, stdout=subprocess.DEVNULL)
    if engine == "podman":
        subprocess.run(
            ["systemctl", "--user", "stop", "openbao-splunk-archive-status"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
    else:
        stop_observer(local)
    print("Stopped only the profile health services; retained fixture files and volumes.")
