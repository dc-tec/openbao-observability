"""Hosted-only synthetic PostgreSQL qualification; never attach existing volumes."""

import hashlib
import json
import os
import subprocess
import tempfile
import time
import uuid
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen

OPENBAO = "quay.io/openbao/openbao:2.7.3"
SOURCE = "postgres:18-alpine"
TARGET = "postgres:19-alpine"
ADMIN = "qualification_admin"
PASSWORD = "synthetic-qualification-only"
TOKEN = "synthetic-openbao-root"
DATABASE = "qualification_app"
ROWS = "1|synthetic-one\n2|synthetic-two"


def require_equal(actual, expected):
    if actual != expected:
        raise AssertionError(f"qualification mismatch: {actual!r} != {expected!r}")


def docker(*args, data=None, check=True):
    result = subprocess.run(
        ["docker", *args],
        input=data,
        capture_output=True,
        timeout=180,
        check=False,
    )
    if check and result.returncode:
        # Do not include commands, dumps, API responses, or credentials in errors.
        raise RuntimeError(f"docker {args[0]} failed with status {result.returncode}")
    return result


def image_identity(image):
    item = json.loads(docker("image", "inspect", image).stdout)[0]
    return {key: item[key] for key in ("Id", "RepoDigests", "Os", "Architecture")}


class Fixture:
    def __init__(self):
        self.prefix = "qual-pg-" + uuid.uuid4().hex
        self.network = self.prefix + "-net"
        self.containers = []
        self.volumes = []
        self.directory = tempfile.TemporaryDirectory(prefix="qual-pg-")
        self.address = None

    def __enter__(self):
        docker("network", "create", "--internal", self.network)
        return self

    def __exit__(self, *_):
        # Only names created by this run enter these lists. No global prune.
        for name in reversed(self.containers):
            docker("unpause", name, check=False)
            docker("rm", "--force", "--volumes", name, check=False)
        for name in self.volumes:
            docker("volume", "rm", name, check=False)
        docker("network", "rm", self.network, check=False)
        self.directory.cleanup()

    def postgres(self, suffix, image, user=ADMIN, database=DATABASE):
        name = self.prefix + "-" + suffix
        volume = name + "-data"
        docker("volume", "create", volume)
        self.volumes.append(volume)
        self.containers.append(name)
        docker(
            "run",
            "--detach",
            "--name",
            name,
            "--network",
            self.network,
            "--network-alias",
            "database",
            "--mount",
            f"type=volume,source={volume},target=/var/lib/postgresql",
            "--env",
            "POSTGRES_USER=" + user,
            "--env",
            "POSTGRES_DB=" + database,
            "--env",
            "POSTGRES_PASSWORD=" + PASSWORD,
            "--env",
            "POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 --auth-local=trust",
            image,
        )
        info = json.loads(docker("inspect", name).stdout)[0]
        require_equal(info["Image"], image_identity(image)["Id"])
        for _ in range(60):
            if (
                self.sql(
                    name, "SELECT 1", user=user, database=database, check=False
                ).returncode
                == 0
            ):
                return name
            time.sleep(1)
        raise RuntimeError(
            "PostgreSQL did not accept TCP connections within 60 seconds"
        )

    def sql(
        self, name, query, user=ADMIN, password=PASSWORD, database=DATABASE, check=True
    ):
        return docker(
            "exec",
            "--env",
            "PGPASSWORD=" + password,
            name,
            "psql",
            "-X",
            "--no-password",
            "-h",
            "127.0.0.1",
            "-U",
            user,
            "-d",
            database,
            "-v",
            "ON_ERROR_STOP=1",
            "-At",
            "-c",
            query,
            check=check,
        )

    def openbao(self):
        config = Path(self.directory.name) / "telemetry.hcl"
        config.write_text(
            'telemetry {\n prometheus_retention_time = "5m"\n disable_hostname = true\n}\n'
        )
        name = self.prefix + "-bao"
        self.containers.append(name)
        docker(
            "run",
            "--detach",
            "--name",
            name,
            "--network",
            self.network,
            "--publish",
            "127.0.0.1::8200",
            "--mount",
            f"type=bind,source={config},target=/bao/config/telemetry.hcl,readonly",
            OPENBAO,
            "server",
            "-dev",
            "-dev-root-token-id=" + TOKEN,
            "-dev-listen-address=0.0.0.0:8200",
            "-config=/bao/config/telemetry.hcl",
        )
        info = json.loads(docker("inspect", name).stdout)[0]
        require_equal(info["Image"], image_identity(OPENBAO)["Id"])
        port = info["NetworkSettings"]["Ports"]["8200/tcp"][0]["HostPort"]
        self.address = "http://127.0.0.1:" + port
        for _ in range(60):
            try:
                require_equal(self.api("sys/health")["version"], "2.7.3")
                return name
            except URLError:
                time.sleep(1)
        raise RuntimeError("OpenBao did not become healthy within 60 seconds")

    def api(self, path, payload=None):
        request = Request(
            self.address + "/v1/" + path,
            data=None if payload is None else json.dumps(payload).encode(),
            headers={"X-Vault-Token": TOKEN, "Content-Type": "application/json"},
        )
        with urlopen(request, timeout=10) as response:
            body = response.read()
            return json.loads(body) if body else None

    def configure(self):
        self.api("sys/mounts/database", {"type": "database"})
        self.api(
            "database/config/postgres",
            {
                "plugin_name": "postgresql-database-plugin",
                "allowed_roles": ["reader"],
                "connection_url": "postgresql://{{username}}:{{password}}@database:5432/"
                + DATABASE
                + "?sslmode=disable",
                "username": ADMIN,
                "password": PASSWORD,
            },
        )
        self.api(
            "database/roles/reader",
            {
                "db_name": "postgres",
                "default_ttl": "1h",
                "max_ttl": "1h",
                "creation_statements": [
                    "CREATE ROLE \"{{name}}\" WITH LOGIN PASSWORD '{{password}}' VALID UNTIL '{{expiration}}';",
                    "GRANT CONNECT ON DATABASE " + DATABASE + ' TO "{{name}}";',
                    'GRANT USAGE ON SCHEMA public TO "{{name}}";',
                    'GRANT SELECT ON public.synthetic_records TO "{{name}}";',
                ],
            },
        )

    def issue(self):
        secret = self.api("database/creds/reader")
        if (
            not secret.get("lease_id")
            or not secret["data"].get("username")
            or not secret["data"].get("password")
        ):
            raise AssertionError("OpenBao returned incomplete database credentials")
        return secret

    def read_rows(self, name, secret):
        credentials = secret["data"]
        result = self.sql(
            name,
            "SELECT id, payload FROM public.synthetic_records ORDER BY id",
            user=credentials["username"],
            password=credentials["password"],
        )
        require_equal(result.stdout.decode().strip(), ROWS)
        bad = self.sql(
            name,
            "SELECT 1",
            user=credentials["username"],
            password="incorrect",
            check=False,
        )
        if bad.returncode == 0 or b"password authentication failed" not in bad.stderr:
            raise AssertionError(
                "incorrect-password check did not return an authentication failure"
            )

    def revoke(self, name, secret):
        self.api(
            "sys/leases/renew", {"lease_id": secret["lease_id"], "increment": 3600}
        )
        self.api("sys/leases/revoke", {"lease_id": secret["lease_id"], "sync": True})
        credentials = secret["data"]
        result = self.sql(
            name,
            "SELECT 1",
            user=credentials["username"],
            password=credentials["password"],
            check=False,
        )
        if (
            result.returncode == 0
            or b"password authentication failed" not in result.stderr
        ):
            raise AssertionError(
                "revoked database credentials still authenticate or did not return an authentication failure"
            )


def scenario(migrate):
    with Fixture() as fixture:
        source = fixture.postgres("source", SOURCE if migrate else TARGET)
        source_version = (
            fixture.sql(source, "SHOW server_version_num").stdout.decode().strip()
        )
        require_equal(source_version[:2], "18" if migrate else "19")
        fixture.sql(
            source,
            "CREATE TABLE public.synthetic_records (id integer PRIMARY KEY, payload text NOT NULL); "
            "INSERT INTO public.synthetic_records VALUES (1, 'synthetic-one'), (2, 'synthetic-two');",
        )
        bao = fixture.openbao()
        fixture.configure()
        secret = fixture.issue()
        fixture.read_rows(source, secret)
        current = source
        hashes = {}
        if migrate:
            # Pause all OpenBao lease writes while dumping roles and database data.
            docker("pause", bao)
            roles = docker(
                "exec", source, "pg_dumpall", "-U", ADMIN, "--globals-only"
            ).stdout
            data = docker(
                "exec", source, "pg_dump", "-U", ADMIN, "-Fc", "--create", DATABASE
            ).stdout
            hashes = {
                "roles_sha256": hashlib.sha256(roles).hexdigest(),
                "database_sha256": hashlib.sha256(data).hexdigest(),
            }
            docker("stop", source)
            docker("network", "disconnect", fixture.network, source)
            # Separate bootstrap user avoids duplicate-role errors; the source volume is never attached.
            current = fixture.postgres(
                "target", TARGET, user="restore_admin", database="restore_control"
            )
            docker(
                "exec",
                "-i",
                current,
                "psql",
                "-X",
                "-U",
                "restore_admin",
                "-d",
                "restore_control",
                "-v",
                "ON_ERROR_STOP=1",
                data=roles,
            )
            docker(
                "exec",
                "-i",
                current,
                "pg_restore",
                "-U",
                "restore_admin",
                "-d",
                "restore_control",
                "--exit-on-error",
                "--create",
                data=data,
            )
            fixture.sql(current, "ANALYZE")
            require_equal(
                fixture.sql(current, "SHOW server_version_num")
                .stdout.decode()
                .strip()[:2],
                "19",
            )
            docker("unpause", bao)
            fixture.read_rows(
                current, secret
            )  # Restored role, password, grants, and rows.
            fixture.api("sys/leases/lookup", {"lease_id": secret["lease_id"]})
            fixture.api(
                "database/reset/postgres", {}
            )  # Reconnect the plugin to the restored database.
            new_secret = fixture.issue()
            fixture.read_rows(current, new_secret)
            fixture.revoke(current, new_secret)
        fixture.revoke(current, secret)
        target_version = (
            fixture.sql(current, "SHOW server_version_num").stdout.decode().strip()
        )
        return {
            "scenario": "dump-restore-18-to-19" if migrate else "fresh-19",
            "openbao_version": "2.7.3",
            "source_server_version_num": source_version,
            "target_server_version_num": target_version,
            "result": "passed",
            "rows": 2,
            "source_volume_reused": False,
            **hashes,
        }


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true":
        raise SystemExit("This container qualification runs only in GitHub Actions.")
    evidence = {"commit": os.environ.get("GITHUB_SHA"), "images": {}, "scenarios": []}
    output = Path("tmp/qualification/postgres19.json")
    output.parent.mkdir(parents=True, exist_ok=True)
    try:
        for image in (OPENBAO, SOURCE, TARGET):
            docker("pull", image)
            evidence["images"][image] = image_identity(image)
        for migrate in (False, True):
            evidence["scenarios"].append(scenario(migrate))
    finally:
        output.write_text(json.dumps(evidence, indent=2) + "\n")
    print(json.dumps(evidence, indent=2))


if __name__ == "__main__":
    main()
