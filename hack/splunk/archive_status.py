"""Report read-back status for the trial's local OTLP audit-file sink."""

import argparse
import json
import time
from pathlib import Path


def observe(directory):
    archive = directory / "audit.jsonl"
    status = {
        "enabled": True,
        "delivery_success": False,
        "last_success_timestamp_seconds": 0,
        "delivery_failures_total": 0,
        "dead_letter_records_total": 0,
        "backend": "qualification-local-file",
        "pipeline": "otel-audit-file",
    }
    try:
        # File exporter emits complete OTLP JSON records separated by newlines.
        complete = archive.read_bytes().split(b"\n")[:-1]
        for raw in reversed(complete):
            if not raw:
                continue
            record = json.loads(raw)
            groups = record.get("resourceLogs", [])
            if any(scope.get("logRecords") for group in groups for scope in group.get("scopeLogs", [])):
                status["delivery_success"] = True
                status["last_success_timestamp_seconds"] = archive.stat().st_mtime
                break
    except (OSError, ValueError):
        pass
    temp = directory / "status.json.tmp"
    temp.write_text(json.dumps(status) + "\n")
    temp.replace(directory / "status.json")
    return status


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--directory", type=Path, default=Path(__file__).resolve().parents[2] / "examples/splunk/.runtime/archive"
    )
    parser.add_argument("--loop", action="store_true")
    args = parser.parse_args()
    args.directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    while True:
        status = observe(args.directory)
        if not args.loop:
            print(json.dumps(status, indent=2))
            return
        time.sleep(5)


if __name__ == "__main__":
    main()
