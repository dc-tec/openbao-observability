"""Hosted-only build and runtime qualification of the existing exporter Dockerfile."""

import hashlib
import io
import json
import os
import re
import subprocess
import tarfile
import tempfile
import time
import uuid
from pathlib import Path
from urllib.error import URLError
from urllib.request import urlopen

CANDIDATE = "golang:1.27.4-alpine"
DOCKERFILE = Path("examples/audit-archive-health/exporter/Dockerfile")
BINARY = "audit-archive-health-exporter"
METRIC_PREFIX = "openbao_audit_archive_"
LABELS = '{backend="qualification",pipeline="synthetic"}'


def docker(*args, check=True):
    result = subprocess.run(
        ["docker", *args], capture_output=True, timeout=600, check=False
    )
    if check and result.returncode:
        raise RuntimeError(
            f"docker {args[0]} failed: {result.stderr.decode(errors='replace')}"
        )
    return result


def inspect_image(image):
    return json.loads(docker("image", "inspect", image).stdout)[0]


def identity(image):
    return {key: image[key] for key in ("Id", "RepoDigests", "Os", "Architecture")}


def candidate_recipe(source, digest):
    first, rest = source.split("\n", 1)
    if first not in (
        "FROM golang:1.27.3-alpine AS build",
        "FROM " + CANDIDATE + " AS build",
    ):
        raise AssertionError(
            "Review the changed Dockerfile builder before qualifying it"
        )
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", digest):
        raise AssertionError("builder digest is not SHA-256")
    return "FROM " + CANDIDATE + "@" + digest + " AS build\n" + rest


def verify_scratch(image, binary):
    if image["Config"]["User"] != "65532:65532":
        raise AssertionError(
            "exporter image does not select the expected non-root user"
        )
    if image["Config"]["Entrypoint"] != ["/" + BINARY]:
        raise AssertionError("exporter entrypoint changed")
    if len(image["RootFS"]["Layers"]) != 1 or binary[:4] != b"\x7fELF":
        raise AssertionError(
            "expected a one-layer scratch image containing an ELF binary"
        )


def image_binary(image):
    # Read archive members in memory; never extract an untrusted archive to disk.
    with tarfile.open(fileobj=io.BytesIO(docker("save", image).stdout)) as archive:
        manifest = json.load(archive.extractfile("manifest.json"))
        if len(manifest) != 1 or len(manifest[0]["Layers"]) != 1:
            raise AssertionError("scratch image must contain exactly one layer")
        layer = archive.extractfile(manifest[0]["Layers"][0]).read()
    with tarfile.open(fileobj=io.BytesIO(layer)) as archive:
        files = [item for item in archive.getmembers() if not item.isdir()]
        if (
            len(files) != 1
            or files[0].name.removeprefix("./") != BINARY
            or not files[0].isfile()
        ):
            raise AssertionError("scratch layer contains files other than the exporter")
        return archive.extractfile(files[0]).read()


def verify_metrics(text, expected):
    observed = {}
    for line in text.splitlines():
        if not line or line.startswith("#"):
            continue
        name, value = line.rsplit(" ", 1)
        if not name.endswith(LABELS):
            raise AssertionError("unexpected exporter metric labels")
        name = name.removesuffix(LABELS)
        if not name.startswith(METRIC_PREFIX):
            raise AssertionError("unexpected exporter metric")
        name = name.removeprefix(METRIC_PREFIX)
        if name in observed:
            raise AssertionError("duplicate exporter metric")
        observed[name] = float(value)
    if observed != expected:
        raise AssertionError(f"metric mismatch: {observed!r} != {expected!r}")


def runtime(image, directory, name):
    status = directory / "state.json"
    docker(
        "run",
        "--detach",
        "--name",
        name,
        "--read-only",
        "--cap-drop",
        "ALL",
        "--security-opt",
        "no-new-privileges",
        "--publish",
        "127.0.0.1::19110",
        "--mount",
        f"type=bind,source={directory},target=/status,readonly",
        image,
        "--enabled",
        "--status-file",
        "/status/state.json",
        "--backend",
        "qualification",
        "--pipeline",
        "synthetic",
    )
    info = json.loads(docker("inspect", name).stdout)[0]
    if info["Image"] != inspect_image(image)["Id"]:
        raise AssertionError("runtime container uses a different image")
    port = info["NetworkSettings"]["Ports"]["19110/tcp"][0]["HostPort"]
    address = "http://127.0.0.1:" + port

    def get(path):
        with urlopen(address + path, timeout=5) as response:
            return response.read().decode()

    for _ in range(30):
        try:
            if get("/healthz") != "ok\n":
                raise AssertionError("unexpected exporter health response")
            break
        except URLError:
            time.sleep(1)
    else:
        raise RuntimeError("exporter did not become healthy within 30 seconds")

    top = docker("top", name, "-eo", "uid,pid,comm").stdout.decode().splitlines()
    if len(top) != 2 or top[1].split()[0] != "65532":
        raise AssertionError("expected one exporter process running as UID 65532")

    healthy = {
        "enabled": 1,
        "delivery_success": 1,
        "last_success_timestamp_seconds": 1720000000,
        "delivery_failures_total": 2,
        "dead_letter_records_total": 3,
    }
    cases = [
        ("healthy", {**healthy, "enabled": True, "delivery_success": True}, healthy),
        (
            "degraded",
            {**healthy, "enabled": True, "delivery_success": False},
            {**healthy, "delivery_success": 0},
        ),
        (
            "malformed",
            "{",
            {
                "enabled": 1,
                "delivery_success": 0,
                "delivery_failures_total": 0,
                "dead_letter_records_total": 0,
            },
        ),
        ("disabled", {"enabled": False}, {"enabled": 0}),
        (
            "missing",
            None,
            {
                "enabled": 1,
                "delivery_success": 0,
                "delivery_failures_total": 0,
                "dead_letter_records_total": 0,
            },
        ),
    ]
    passed = []
    for case, content, expected in cases:
        if content is None:
            status.unlink(missing_ok=True)
        else:
            replacement = directory / "next.json"
            replacement.write_text(
                content if isinstance(content, str) else json.dumps(content)
            )
            replacement.chmod(0o644)
            replacement.replace(status)
        verify_metrics(get("/metrics"), expected)
        if get("/healthz") != "ok\n":
            raise AssertionError("exporter failed after status replacement")
        passed.append(case)
    docker("stop", "--time", "3", name)
    state = json.loads(docker("inspect", name).stdout)[0]["State"]
    if state["Running"] or state["ExitCode"] not in (0, 143) or state["OOMKilled"]:
        raise AssertionError("exporter did not stop on SIGTERM without SIGKILL or OOM")
    invalid = docker(
        "run",
        "--rm",
        "--network",
        "none",
        image,
        "--metrics-path",
        "relative",
        check=False,
    )
    if (
        invalid.returncode == 0
        or b"--metrics-path must start with /" not in invalid.stderr
    ):
        raise AssertionError(
            "invalid flags did not terminate the exporter with an error"
        )
    return {
        "metrics_cases": passed,
        "uid": 65532,
        "sigterm_exit_code": state["ExitCode"],
        "invalid_flags": "rejected",
    }


def qualify(directory, prefix, evidence):
    docker("pull", "--platform", "linux/amd64", CANDIDATE)
    base = inspect_image(CANDIDATE)
    if (base["Os"], base["Architecture"]) != ("linux", "amd64"):
        raise AssertionError("qualification requires the Linux amd64 builder")
    digest = next(
        ref.split("@", 1)[1] for ref in base["RepoDigests"] if ref.startswith("golang@")
    )
    evidence["builder_base"] = identity(base)
    source = DOCKERFILE.read_text()
    recipe = candidate_recipe(source, digest)
    candidate = directory / "Dockerfile"
    candidate.write_text(recipe)
    evidence["source_dockerfile_sha256"] = hashlib.sha256(source.encode()).hexdigest()
    evidence["candidate_dockerfile_sha256"] = hashlib.sha256(
        recipe.encode()
    ).hexdigest()
    builder, image = prefix + ":builder", prefix + ":scratch"
    docker(
        "build",
        "--pull",
        "--no-cache",
        "--file",
        str(candidate),
        "--target",
        "build",
        "--tag",
        builder,
        ".",
    )
    built = inspect_image(builder)
    layers = base["RootFS"]["Layers"]
    if built["RootFS"]["Layers"][: len(layers)] != layers:
        raise AssertionError(
            "build stage does not inherit the resolved candidate layers"
        )
    version = (
        docker(
            "run",
            "--rm",
            "--network",
            "none",
            "--env",
            "GOTOOLCHAIN=local",
            builder,
            "go",
            "version",
        )
        .stdout.decode()
        .strip()
    )
    if version != "go version go1.27.4 linux/amd64":
        raise AssertionError("unexpected actual builder Go version: " + version)
    alpine = (
        docker(
            "run", "--rm", "--network", "none", builder, "cat", "/etc/alpine-release"
        )
        .stdout.decode()
        .strip()
    )
    metadata = docker(
        "run",
        "--rm",
        "--network",
        "none",
        "--env",
        "GOTOOLCHAIN=local",
        builder,
        "go",
        "version",
        "-m",
        "/out/" + BINARY,
    ).stdout.decode()
    if (
        ": go1.27.4\n" not in metadata
        or "CGO_ENABLED=0" not in metadata
        or "GOOS=linux" not in metadata
        or "GOARCH=amd64" not in metadata
    ):
        raise AssertionError(
            "binary build metadata does not match the qualified builder"
        )
    evidence.update(
        builder=identity(built),
        go_version=version,
        alpine_version=alpine,
        binary_metadata=metadata,
    )
    docker(
        "run",
        "--rm",
        "--network",
        "none",
        "--env",
        "GOTOOLCHAIN=local",
        builder,
        "go",
        "test",
        "./examples/audit-archive-health/exporter",
    )
    evidence["existing_exporter_tests"] = "passed"
    docker("build", "--file", str(candidate), "--tag", image, ".")
    final = inspect_image(image)
    binary = image_binary(image)
    verify_scratch(final, binary)
    digest = hashlib.sha256(binary).hexdigest()
    expected = (
        docker(
            "run", "--rm", "--network", "none", builder, "sha256sum", "/out/" + BINARY
        )
        .stdout.decode()
        .split()[0]
    )
    if digest != expected:
        raise AssertionError(
            "scratch binary differs from the verified build-stage binary"
        )
    evidence.update(scratch=identity(final), binary_sha256=digest)
    evidence["runtime"] = runtime(image, directory, prefix + "-runtime")
    if DOCKERFILE.read_text() != source:
        raise AssertionError("qualification changed the source Dockerfile")
    evidence["result"] = "passed"


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise SystemExit("This container qualification runs only in GitHub Actions.")
    prefix = "qual-exporter-" + uuid.uuid4().hex
    evidence = {"commit": os.environ.get("GITHUB_SHA"), "candidate": CANDIDATE}
    output = Path("tmp/qualification/exporter-builder.json")
    output.parent.mkdir(parents=True, exist_ok=True)
    try:
        with tempfile.TemporaryDirectory(prefix="qual-exporter-") as temporary:
            directory = Path(temporary)
            directory.chmod(0o755)
            qualify(directory, prefix, evidence)
    finally:
        docker("rm", "--force", prefix + "-runtime", check=False)
        docker("image", "rm", prefix + ":builder", prefix + ":scratch", check=False)
        output.write_text(json.dumps(evidence, indent=2) + "\n")
    print(json.dumps(evidence, indent=2))


if __name__ == "__main__":
    main()
