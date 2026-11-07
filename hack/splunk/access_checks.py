"""Qualify temporary non-admin accounts in the disposable trial, with ACL rollback."""

import configparser
import json
import secrets
import urllib.parse
import uuid

from support import write_private

APP = "/servicesNS/nobody/openbao_observability"


def save(path, value):
    write_private(path, json.dumps(value, indent=2) + "\n")


def cleanup(local):
    path = local.DIRECTORY / ".runtime/access-private.json"
    if not path.exists():
        return
    state = json.loads(path.read_text())
    auth = local.session()
    for item in reversed(state["acls"]):
        acl = item["original"]
        local.request(
            item["path"] + "/acl",
            {
                "owner": acl["owner"],
                "sharing": acl["sharing"],
                "perms.read": ",".join(acl["perms"]["read"]),
                "perms.write": ",".join(acl["perms"]["write"]),
            },
            auth,
        )
    for account in state["accounts"]:
        for base, name in [
            ("/services/authentication/users/", account["user"]),
            ("/services/authorization/roles/", account["role"]),
        ]:
            try:
                local.delete(base + name, auth)
            except local.APIError as error:
                if error.status != 404:
                    raise
    path.unlink()
    print("Removed temporary qualification accounts and restored app/object ACLs.")


def run(local, keep_for_browser=False):
    completed = False
    try:
        qualify(local, keep_for_browser)
        completed = True
    finally:
        if not completed or not keep_for_browser:
            cleanup(local)


def qualify(local, keep_for_browser):
    private = local.DIRECTORY / ".runtime/access-private.json"
    if private.exists():
        cleanup(local)
    auth = local.session()
    suffix = uuid.uuid4().hex[:8]
    state = {"accounts": [], "acls": []}
    save(private, state)
    all_indexes = ["openbao_metrics", "openbao_operational", "openbao_audit", "openbao_completed", "openbao_platform"]
    ops_indexes = ["openbao_metrics", "openbao_operational", "openbao_platform"]
    for kind, indexes in [("reader", all_indexes), ("operator", ops_indexes)]:
        name = "openbao_qualification_" + kind + "_" + suffix
        account = {"kind": kind, "role": name, "user": name, "password": secrets.token_urlsafe(32)}
        state["accounts"].append(account)
        save(private, state)
        local.request(
            "/services/authorization/roles",
            {
                "name": name,
                "capabilities": ["search", "rest_apps_view", "get_metadata", "get_typeahead"],
                "srchIndexesAllowed": indexes,
                "srchIndexesDefault": ["openbao_metrics"],
                "srchJobsQuota": 16,
                "srchDiskQuota": 256,
                "defaultApp": "openbao_observability",
            },
            auth,
        )
        local.request(
            "/services/authentication/users",
            {
                "name": name,
                "password": account["password"],
                "roles": [name],
                "force-change-pass": False,
                "defaultApp": "openbao_observability",
            },
            auth,
        )
        role = local.request("/services/authorization/roles/" + name, auth=auth)["entry"][0]["content"]
        if set(role["srchIndexesAllowed"]) != set(indexes) or role.get("imported_roles"):
            raise RuntimeError("Qualification role inherited unexpected indexes or roles")
    reader, operator = state["accounts"]
    ops_views = {
        "openbao_ha_raft",
        "openbao_runtime_storage",
        "openbao_operational_logs",
        "openbao_kubernetes_platform",
        "openbao_slo_availability",
    }
    paths = [("/services/apps/local/openbao_observability", True)]
    app = local.ROOT / "generated/splunk/app/openbao_observability"
    config = configparser.ConfigParser(interpolation=None)
    config.read(app / "default/macros.conf")
    for name in config.sections():
        ops = not name.startswith(("openbao_audit_", "openbao_completed_"))
        paths.append((APP + "/configs/conf-macros/" + urllib.parse.quote(name, safe=""), ops))
    for view in (app / "default/data/ui/views").glob("*.xml"):
        paths.append((APP + "/data/ui/views/" + view.stem, view.stem in ops_views))
    paths.append((APP + "/data/ui/nav/default", True))
    for path, ops in paths:
        acl = local.request(path, auth=auth)["entry"][0]["acl"]
        state["acls"].append({"path": path, "original": acl})
        save(private, state)
        reads = list(dict.fromkeys(acl["perms"]["read"] + [reader["role"]] + ([operator["role"]] if ops else [])))
        local.request(
            path + "/acl",
            {"owner": acl["owner"], "sharing": "app", "perms.read": ",".join(reads), "perms.write": "admin"},
            auth,
        )
    reader_auth = local.session(reader["user"], reader["password"])
    operator_auth = local.session(operator["user"], operator["password"])
    viewed = 0
    for view in (app / "default/data/ui/views").glob("*.xml"):
        local.request(
            "/servicesNS/" + reader["user"] + "/openbao_observability/data/ui/views/" + view.stem, auth=reader_auth
        )
        viewed += 1
    local.search(
        '| `openbao_metric_samples("vault_core_active")` | stats count AS count',
        auth=operator_auth,
        username=operator["user"],
    )
    audit = local.search(
        "`openbao_audit_events` | stats count AS count", earliest="-24h", auth=reader_auth, username=reader["user"]
    )
    if not audit or int(audit[0]["count"]) == 0:
        raise RuntimeError("Reader positive audit control has no data")
    denied = []
    for index in ("openbao_audit", "openbao_completed", "main", "_internal"):
        try:
            rows = local.search(
                "index=" + index + " | stats count AS count",
                earliest="-24h",
                auth=operator_auth,
                username=operator["user"],
            )
        except RuntimeError as error:
            if not any(value in str(error) for value in ("permission", "not allowed", "No matching", "HTTP 403")):
                raise
            rows = []
        if rows and int(rows[0].get("count", 0)) != 0:
            raise RuntimeError("Operator accessed forbidden index " + index)
        denied.append(index)
    view_path = APP + "/data/ui/views/openbao_overview"
    try:
        local.request(
            view_path, {"eai:data": (app / "default/data/ui/views/openbao_overview.xml").read_text()}, reader_auth
        )
    except RuntimeError as error:
        if "HTTP 403" not in str(error) and "HTTP 404" not in str(error):
            raise
    else:
        raise RuntimeError("Read-only account unexpectedly updated a dashboard")
    role = local.request("/services/authorization/roles/" + reader["role"], auth=auth)["entry"][0]["content"]
    forbidden_caps = {"admin_all_objects", "edit_roles", "edit_user", "indexes_edit", "schedule_search"}
    if forbidden_caps.intersection(role.get("capabilities", []) + role.get("imported_capabilities", [])):
        raise RuntimeError("Read-only account received a privileged capability")
    result = {
        "views_read": viewed,
        "audit_positive_control": True,
        "dashboard_write_denied": True,
        "operator_denied_indexes": denied,
        "index_allowlists_verified": True,
        "temporary_accounts": "Retained for browser; run access-cleanup."
        if keep_for_browser
        else "Removed after qualification.",
    }
    save(local.DIRECTORY / ".runtime/access.json", result)
    print(json.dumps(result, indent=2))
