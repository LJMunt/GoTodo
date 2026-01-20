"""
End-to-end API smoke test for GoTodo.

Runs auth + admin lifecycle tests.
Intended for local dev and CI.
"""

from __future__ import annotations

import argparse
import json
import sys
import time
import uuid
from dataclasses import dataclass
from typing import Any, Optional, Tuple
from urllib.error import HTTPError, URLError
from urllib.parse import urljoin
from urllib.request import Request, urlopen


class TestFailure(Exception):
    pass


def _now_ms() -> int:
    return int(time.time() * 1000)


def _json_dumps(obj: Any) -> bytes:
    return json.dumps(obj, separators=(",", ":"), ensure_ascii=False).encode("utf-8")


@dataclass
class HTTPResponse:
    status: int
    headers: dict[str, str]
    body_bytes: bytes

    def json(self) -> Any:
        if not self.body_bytes:
            return None
        return json.loads(self.body_bytes.decode("utf-8"))


class HttpClient:
    def __init__(self, base_url: str, timeout_sec: float = 10.0) -> None:
        self.base_url = base_url if base_url.endswith("/") else base_url + "/"
        self.timeout_sec = timeout_sec

    def request(
        self,
        method: str,
        path: str,
        token: Optional[str] = None,
        json_body: Optional[dict[str, Any]] = None,
        extra_headers: Optional[dict[str, str]] = None,
    ) -> HTTPResponse:
        url = urljoin(self.base_url, path.lstrip("/"))

        headers = {
            "Accept": "application/json",
        }
        if json_body is not None:
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = f"Bearer {token}"
        if extra_headers:
            headers.update(extra_headers)

        data = _json_dumps(json_body) if json_body is not None else None
        req = Request(url=url, method=method.upper(), headers=headers, data=data)

        try:
            with urlopen(req, timeout=self.timeout_sec) as resp:
                body = resp.read()  # bytes
                return HTTPResponse(
                    status=getattr(resp, "status", 0),
                    headers={k.lower(): v for k, v in resp.headers.items()},
                    body_bytes=body,
                )
        except HTTPError as e:
            # HTTPError is also a file-like response
            body = e.read() if hasattr(e, "read") else b""
            return HTTPResponse(
                status=e.code,
                headers={k.lower(): v for k, v in e.headers.items()} if e.headers else {},
                body_bytes=body,
            )
        except URLError as e:
            raise TestFailure(f"Network/URL error calling {method} {url}: {e}") from e


def expect_status(resp: HTTPResponse, expected: int, label: str) -> None:
    if resp.status != expected:
        # include response body if possible
        body_preview = resp.body_bytes.decode("utf-8", errors="replace")
        raise TestFailure(
            f"{label}: expected HTTP {expected}, got {resp.status}. Body: {body_preview}"
        )

def expect_status_in(resp: HTTPResponse, expected: Tuple[int, ...], label: str) -> None:
    if resp.status not in expected:
        body_preview = resp.body_bytes.decode("utf-8", errors="replace")
        raise TestFailure(
            f"{label}: expected HTTP {expected}, got {resp.status}. Body: {body_preview}"
        )


@dataclass
class Tokens:
    user_token: str
    admin_token: str


@dataclass
class CreatedUser:
    email: str
    password: str
    new_password: str
    user_id: int


@dataclass
class Project:
    id: int
    name: str
    description: Optional[str] = None
    created_at: Optional[str] = None
    updated_at: Optional[str] = None


class GoToDoAPI:

    def __init__(self, base_url: str, timeout_sec: float = 10.0) -> None:
        base = base_url.rstrip("/") + "/"
        self.http = HttpClient(base, timeout_sec=timeout_sec)

    # ---- public endpoints ----
    def health(self) -> HTTPResponse:
        return self.http.request("GET", "/api/v1/health")

    def ready(self) -> HTTPResponse:
        return self.http.request("GET", "/api/v1/ready")

    def signup(self, email: str, password: str) -> HTTPResponse:
        return self.http.request(
            "POST",
            "/api/v1/auth/signup",
            json_body={"email": email, "password": password},
        )

    def login(self, email: str, password: str) -> HTTPResponse:
        return self.http.request(
            "POST",
            "/api/v1/auth/login",
            json_body={"email": email, "password": password},
        )

    # ---- authenticated ----
    def me(self, token: str) -> HTTPResponse:
        return self.http.request("GET", "/api/v1/users/me", token=token)

    # ---- admin ----
    def admin_list_users(self, admin_token: str) -> HTTPResponse:
        return self.http.request("GET", "/api/v1/admin/users", token=admin_token)

    def admin_get_user(self, admin_token: str, user_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/admin/users/{user_id}", token=admin_token)

    def admin_update_user(
        self, admin_token: str, user_id: int, patch: dict[str, Any]
    ) -> HTTPResponse:
        return self.http.request(
            "PATCH",
            f"/api/v1/admin/users/{user_id}",
            token=admin_token,
            json_body=patch,
        )

    def admin_disable_user(self, admin_token: str, user_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/admin/users/{user_id}", token=admin_token)

    # ---- projects ----
    def create_project(self, token: str, name: str, description: Optional[str] = None) -> HTTPResponse:
        body = {"name": name}
        if description:
            body["description"] = description
        return self.http.request("POST", "/api/v1/projects", token=token, json_body=body)

    def list_projects(self, token: str) -> HTTPResponse:
        return self.http.request("GET", "/api/v1/projects", token=token)

    def get_project(self, token: str, project_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/projects/{project_id}", token=token)

    def update_project(self, token: str, project_id: int, patch: dict[str, Any]) -> HTTPResponse:
        return self.http.request("PATCH", f"/api/v1/projects/{project_id}", token=token, json_body=patch)

    def delete_project(self, token: str, project_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/projects/{project_id}", token=token)

    # ---- admin projects ----
    def admin_list_user_projects(self, admin_token: str, user_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/admin/users/{user_id}/projects", token=admin_token)

    def admin_get_user_project(self, admin_token: str, user_id: int, project_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/admin/users/{user_id}/projects/{project_id}", token=admin_token)

    def admin_update_user_project(self, admin_token: str, user_id: int, project_id: int, patch: dict[str, Any]) -> HTTPResponse:
        return self.http.request("PATCH", f"/api/v1/admin/users/{user_id}/projects/{project_id}", token=admin_token, json_body=patch)

    def admin_delete_user_project(self, admin_token: str, user_id: int, project_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/admin/users/{user_id}/projects/{project_id}", token=admin_token)

    def wait_ready(self, timeout_sec: float = 10.0, interval_sec: float = 0.25) -> None:
        deadline = time.time() + timeout_sec
        last_status = None
        while time.time() < deadline:
            resp = self.ready()
            last_status = resp.status
            if resp.status == 200:
                return
            time.sleep(interval_sec)
        raise TestFailure(f"/ready did not become 200 within {timeout_sec}s (last status: {last_status})")


def log(msg: str) -> None:
    print(msg)


class SmokeTestRunner:
    def __init__(self, api: GoToDoAPI, verbose: bool = False) -> None:
        self.api = api
        self.verbose = verbose

    def vlog(self, msg: str) -> None:
        if self.verbose:
            print(msg)

    def run(
        self,
        admin_email: str,
        admin_password: str,
        user_email_prefix: str,
        user_password: str,
        user_new_password: str,
    ) -> None:
        log("== GoToDo API smoke test ==")
        log("Waiting for API readiness...")
        self.api.wait_ready(timeout_sec=10.0)


    # 1) Health/ready
        log("[1/22] health + ready")
        expect_status(self.api.health(), 200, "GET /health")
        expect_status(self.api.ready(), 200, "GET /ready")

        # 2) Login admin (admin must already exist and be admin in DB)
        log("[2/22] admin login")
        admin_login = self.api.login(admin_email, admin_password)
        expect_status(admin_login, 200, "POST /auth/login (admin)")
        admin_token = admin_login.json().get("token")
        if not admin_token:
            raise TestFailure("Admin login did not return a token")

        # 3) Create unique user
        uniq = f"{_now_ms()}-{uuid.uuid4().hex[:10]}"
        user_email = f"{user_email_prefix}+{uniq}@example.com" if "@" not in user_email_prefix else user_email_prefix.replace("@", f"+{uniq}@")
        log(f"[3/22] signup user: {user_email}")
        signup = self.api.signup(user_email, user_password)
        # Depending on your signup handler, you may return 201 with user json (and possibly a token).
        expect_status(signup, 201, "POST /auth/signup (user)")

        # 4) Login as the new user
        log("[4/22] user login")
        user_login = self.api.login(user_email, user_password)
        expect_status(user_login, 200, "POST /auth/login (user)")
        user_token = user_login.json().get("token")
        if not user_token:
            raise TestFailure("User login did not return a token")

        # 5) /me should work and return id
        log("[5/22] GET /users/me")
        me = self.api.me(user_token)
        expect_status(me, 200, "GET /users/me")
        me_json = me.json()
        self.vlog(f"/me response: {me_json}")
        user_id = me_json.get("id")
        if not isinstance(user_id, int):
            raise TestFailure(f"/me did not return integer id. Got: {user_id}")

        # 6) Admin disables user (soft delete)
        log("[6/22] admin disables user (soft delete)")
        disable = self.api.admin_disable_user(admin_token, user_id)
        expect_status(disable, 204, "DELETE /admin/users/{id}")

        # 7) Disabled user should fail login (your login returns 401 for invalid/disabled)
        log("[7/22] disabled user cannot login")
        disabled_login = self.api.login(user_email, user_password)
        expect_status(disabled_login, 401, "POST /auth/login (disabled user)")

        # 8) Admin re-enables user
        log("[8/22] admin re-enables user")
        enable = self.api.admin_update_user(admin_token, user_id, {"is_active": True})
        expect_status_in(enable, (200, 204), "PATCH /admin/users/{id} (enable)")
        self.vlog(f"enable response: {enable.json()}")

        # 9) User can login again, then admin changes password, then user can login with new password
        log("[9/22] password change cycle")
        user_login2 = self.api.login(user_email, user_password)
        expect_status(user_login2, 200, "POST /auth/login (re-enabled user)")
        user_token2 = user_login2.json().get("token")
        if not user_token2:
            raise TestFailure("Re-enabled user login did not return a token")

        pw_change = self.api.admin_update_user(admin_token, user_id, {"password": user_new_password})
        expect_status_in(pw_change, (200, 204), "PATCH /admin/users/{id} (password)")
        self.vlog(f"pw change response: {pw_change.json()}")

        # old password should fail now (401)
        old_pw_login = self.api.login(user_email, user_password)
        expect_status(old_pw_login, 401, "POST /auth/login (old password should fail)")

        # new password should succeed
        new_pw_login = self.api.login(user_email, user_new_password)
        expect_status(new_pw_login, 200, "POST /auth/login (new password)")
        user_token3 = new_pw_login.json().get("token")
        if not user_token3:
            raise TestFailure("New password login did not return a token")

        # 10) Non-admin user must not access admin endpoint
        log("[10/22] user forbidden from admin endpoints")
        admin_list_as_user = self.api.admin_list_users(user_token3)
        expect_status(admin_list_as_user, 403, "GET /admin/users (as non-admin)")

        # 11) smoke-test user makes new project
        log("[11/22] user creates project")
        create_proj = self.api.create_project(user_token3, "My Project v1", "My Description")
        expect_status(create_proj, 201, "POST /projects")
        proj_data = create_proj.json()
        if not proj_data.get("id"):
            raise TestFailure("Create project did not return an id")
        project1 = Project(
            id=proj_data["id"],
            name=proj_data["name"],
            description=proj_data.get("description"),
            created_at=proj_data.get("created_at"),
            updated_at=proj_data.get("updated_at")
        )

        # 12) smoke-test user gets project
        log("[12/22] user gets project")
        get_proj = self.api.get_project(user_token3, project1.id)
        expect_status(get_proj, 200, "GET /projects/{id}")
        self.vlog(f"get project response: {get_proj.json()}")

        # 13) smoke-test user lists all project
        log("[13/22] user lists projects")
        list_projs = self.api.list_projects(user_token3)
        expect_status(list_projs, 200, "GET /projects")
        projs = list_projs.json()
        if not any(p["id"] == project1.id for p in projs):
            raise TestFailure(f"Project {project1.id} not found in project list")

        # 14) smoke-test user updates project
        log("[14/22] user updates project")
        update_proj = self.api.update_project(user_token3, project1.id, {"name": "Updated Project"})
        expect_status_in(update_proj, (200, 204), "PATCH /projects/{id}")

        # 15) smoke-test user deletes project
        log("[15/22] user deletes project")
        delete_proj = self.api.delete_project(user_token3, project1.id)
        expect_status_in(delete_proj, (200, 204), "DELETE /projects/{id}")

        # 16) smoke-test user gets project -> must fail
        log("[16/22] user gets deleted project (must fail)")
        get_deleted_proj = self.api.get_project(user_token3, project1.id)
        expect_status(get_deleted_proj, 404, "GET /projects/{id} (deleted)")

        # 17) smoke-test user makes new project
        log("[17/22] user creates another project")
        create_proj2 = self.api.create_project(user_token3, "Admin Test Project")
        expect_status(create_proj2, 201, "POST /projects")
        proj_data2 = create_proj2.json()
        project2 = Project(
            id=proj_data2["id"],
            name=proj_data2["name"],
            description=proj_data2.get("description"),
            created_at=proj_data2.get("created_at"),
            updated_at=proj_data2.get("updated_at")
        )

        # 18) admin lists projects
        log("[18/22] admin lists user's projects")
        admin_list_projs = self.api.admin_list_user_projects(admin_token, user_id)
        expect_status(admin_list_projs, 200, "GET /admin/users/{userId}/projects")
        admin_projs = admin_list_projs.json()
        if not any(p["id"] == project2.id for p in admin_projs):
            raise TestFailure(f"Project {project2.id} not found in admin list of user projects")

        # 19) admin gets the project
        log("[19/22] admin gets user's project")
        admin_get_proj = self.api.admin_get_user_project(admin_token, user_id, project2.id)
        expect_status(admin_get_proj, 200, "GET /admin/users/{userId}/projects/{projectId}")

        # 20) admin updates project
        log("[20/22] admin updates user's project")
        admin_update_proj = self.api.admin_update_user_project(admin_token, user_id, project2.id, {"description": "Admin updated this"})
        expect_status_in(admin_update_proj, (200, 204), "PATCH /admin/users/{userId}/projects/{projectId}")

        # 21) admin deletes project
        log("[21/22] admin deletes user's project")
        admin_delete_proj = self.api.admin_delete_user_project(admin_token, user_id, project2.id)
        expect_status_in(admin_delete_proj, (200, 204), "DELETE /admin/users/{userId}/projects/{projectId}")

        # 22) admin gets project -> must fail
        log("[22/22] admin gets deleted project (must fail)")
        admin_get_deleted_proj = self.api.admin_get_user_project(admin_token, user_id, project2.id)
        expect_status(admin_get_deleted_proj, 404, "GET /admin/users/{userId}/projects/{projectId} (deleted)")

        log("✅ All smoke tests passed.")


def main() -> int:
    p = argparse.ArgumentParser(
        description="GoToDo API smoke test (auth + admin user lifecycle). Exits non-zero on failure."
    )
    p.add_argument("--wait-ready-seconds", type=float, default=10.0, help="Wait up to N seconds for /ready to return 200")
    p.add_argument("--base-url", required=True, help="Base URL, e.g. http://localhost:8081")
    p.add_argument("--admin-email", required=True, help="Existing admin email")
    p.add_argument("--admin-password", required=True, help="Existing admin password")
    p.add_argument(
        "--user-email-prefix",
        default="smoketest",
        help="Prefix for generated user email. Default: smoketest (creates smoketest+<uniq>@example.com). "
             "If you pass a full email, it will inject +<uniq> before @.",
    )
    p.add_argument("--user-password", default="password123", help="Password for created user (default: password123)")
    p.add_argument("--user-new-password", default="newpassword123", help="New password set by admin (default: newpassword123)")
    p.add_argument("--timeout", type=float, default=10.0, help="HTTP timeout seconds (default: 10)")
    p.add_argument("--verbose", action="store_true", help="Print response JSON for some steps")
    args = p.parse_args()

    api = GoToDoAPI(args.base_url, timeout_sec=args.timeout)
    runner = SmokeTestRunner(api, verbose=args.verbose)

    try:
        api.wait_ready(timeout_sec=args.wait_ready_seconds)
        runner.run(
            admin_email=args.admin_email,
            admin_password=args.admin_password,
            user_email_prefix=args.user_email_prefix,
            user_password=args.user_password,
            user_new_password=args.user_new_password,
        )
        return 0
    except TestFailure as e:
        print(f"❌ TEST FAILED: {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
