"""
End-to-end API smoke test for GoTodo.

Runs auth + admin lifecycle tests.
Intended for local dev and CI.

Run Config for Dev in Goland:
1. Create test@example.com user via API
2. Set this user as admin
3. Use these parameters:
--base-url http://localhost:8081 --admin-email test@example.com --admin-password password123
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


@dataclass
class Task:
    id: int
    title: str
    description: Optional[str] = None
    is_completed: bool = False
    project_id: Optional[int] = None
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

    def admin_restore_user_project(self, admin_token: str, user_id: int, project_id: int) -> HTTPResponse:
        return self.http.request("POST", f"/api/v1/admin/users/{user_id}/projects/{project_id}/restore", token=admin_token)

    # ---- tasks ----
    def create_task(self, token: str, project_id: int, title: str, description: Optional[str] = None, **kwargs) -> HTTPResponse:
        body = {"title": title}
        if description:
            body["description"] = description
        body.update(kwargs)
        return self.http.request("POST", f"/api/v1/projects/{project_id}/tasks", token=token, json_body=body)

    def list_project_tasks(self, token: str, project_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/projects/{project_id}/tasks", token=token)

    def get_task(self, token: str, task_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/tasks/{task_id}", token=token)

    def update_task(self, token: str, task_id: int, patch: dict[str, Any]) -> HTTPResponse:
        return self.http.request("PATCH", f"/api/v1/tasks/{task_id}", token=token, json_body=patch)

    def delete_task(self, token: str, task_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/tasks/{task_id}", token=token)

    # ---- admin tasks ----
    def admin_restore_user_task(self, admin_token: str, user_id: int, task_id: int) -> HTTPResponse:
        return self.http.request("POST", f"/api/v1/admin/users/{user_id}/tasks/{task_id}/restore", token=admin_token)

    def admin_delete_user_task(self, admin_token: str, user_id: int, task_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/admin/users/{user_id}/tasks/{task_id}", token=admin_token)

    # ---- tags ----
    def create_tag(self, token: str, name: str) -> HTTPResponse:
        return self.http.request("POST", "/api/v1/tags", token=token, json_body={"name": name})

    def list_tags(self, token: str) -> HTTPResponse:
        return self.http.request("GET", "/api/v1/tags", token=token)

    def update_tag(self, token: str, tag_id: int, name: str) -> HTTPResponse:
        return self.http.request("PATCH", f"/api/v1/tags/{tag_id}", token=token, json_body={"name": name})

    def delete_tag(self, token: str, tag_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/tags/{tag_id}", token=token)

    def list_task_tags(self, token: str, task_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/tasks/{task_id}/tags", token=token)

    def update_task_tags(self, token: str, task_id: int, tag_ids: list[int]) -> HTTPResponse:
        return self.http.request("PUT", f"/api/v1/tasks/{task_id}/tags", token=token, json_body={"tag_ids": tag_ids})

    def admin_list_user_tags(self, admin_token: str, user_id: int) -> HTTPResponse:
        return self.http.request("GET", f"/api/v1/admin/users/{user_id}/tags", token=admin_token)

    def admin_delete_user_tag(self, admin_token: str, user_id: int, tag_id: int) -> HTTPResponse:
        return self.http.request("DELETE", f"/api/v1/admin/users/{user_id}/tags/{tag_id}", token=admin_token)

    def list_occurrences(self, token: str, task_id: int, from_time: Optional[str] = None, to_time: Optional[str] = None) -> HTTPResponse:
        path = f"/api/v1/tasks/{task_id}/occurrences"
        params = []
        if from_time: params.append(f"from={from_time}")
        if to_time: params.append(f"to={to_time}")
        if params:
            path += "?" + "&".join(params)
        return self.http.request("GET", path, token=token)

    def update_occurrence(self, token: str, task_id: int, occurrence_id: int, completed: bool) -> HTTPResponse:
        return self.http.request("PATCH", f"/api/v1/tasks/{task_id}/occurrences/{occurrence_id}", token=token, json_body={"completed": completed})

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


@dataclass
class TestContext:
    admin_token: str
    user_token: str
    user_id: int
    user_email: str


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

        ctx = self.test_user_lifecycle(
            admin_email, admin_password, user_email_prefix, user_password, user_new_password
        )
        try:
            self.test_project_management(ctx)
            self.test_task_management(ctx)
            self.test_recurrence(ctx)
        finally:
            log("Cleaning up: setting smoketest user as inactive")
            self.api.admin_update_user(ctx.admin_token, ctx.user_id, {"is_active": False})

        log("✅ All smoke tests passed.")

    def test_user_lifecycle(
        self,
        admin_email: str,
        admin_password: str,
        user_email_prefix: str,
        user_password: str,
        user_new_password: str,
    ) -> TestContext:
        # 1) Health/ready
        log("[1/43] health + ready")
        expect_status(self.api.health(), 200, "GET /health")
        expect_status(self.api.ready(), 200, "GET /ready")

        # 2) Login admin (admin must already exist and be admin in DB)
        log("[2/43] admin login")
        admin_login = self.api.login(admin_email, admin_password)
        expect_status(admin_login, 200, "POST /auth/login (admin)")
        admin_token = admin_login.json().get("token")
        if not admin_token:
            raise TestFailure("Admin login did not return a token")

        # 3) Create unique user
        uniq = f"{_now_ms()}-{uuid.uuid4().hex[:10]}"
        user_email = (
            f"{user_email_prefix}+{uniq}@example.com"
            if "@" not in user_email_prefix
            else user_email_prefix.replace("@", f"+{uniq}@")
        )
        log(f"[3/43] signup user: {user_email}")
        signup = self.api.signup(user_email, user_password)
        # Depending on your signup handler, you may return 201 with user json (and possibly a token).
        expect_status(signup, 201, "POST /auth/signup (user)")

        # 4) Login as the new user
        log("[4/43] user login")
        user_login = self.api.login(user_email, user_password)
        expect_status(user_login, 200, "POST /auth/login (user)")
        user_token = user_login.json().get("token")
        if not user_token:
            raise TestFailure("User login did not return a token")

        # 5) /me should work and return id
        log("[5/43] GET /users/me")
        me = self.api.me(user_token)
        expect_status(me, 200, "GET /users/me")
        me_json = me.json()
        self.vlog(f"/me response: {me_json}")
        user_id = me_json.get("id")
        if not isinstance(user_id, int):
            raise TestFailure(f"/me did not return integer id. Got: {user_id}")

        # 6) Admin disables user (soft delete)
        log("[6/43] admin disables user (soft delete)")
        disable = self.api.admin_disable_user(admin_token, user_id)
        expect_status(disable, 204, "DELETE /admin/users/{id}")

        # 7) Disabled user should fail login (your login returns 401 for invalid/disabled)
        log("[7/43] disabled user cannot login")
        disabled_login = self.api.login(user_email, user_password)
        expect_status(disabled_login, 401, "POST /auth/login (disabled user)")

        # 8) Admin re-enables user
        log("[8/43] admin re-enables user")
        enable = self.api.admin_update_user(admin_token, user_id, {"is_active": True})
        expect_status_in(enable, (200, 204), "PATCH /admin/users/{id} (enable)")
        self.vlog(f"enable response: {enable.json()}")

        # 9) User can login again, then admin changes password, then user can login with new password
        log("[9/43] password change cycle")
        user_login2 = self.api.login(user_email, user_password)
        expect_status(user_login2, 200, "POST /auth/login (re-enabled user)")
        user_token2 = user_login2.json().get("token")
        if not user_token2:
            raise TestFailure("Re-enabled user login did not return a token")

        pw_change = self.api.admin_update_user(
            admin_token, user_id, {"password": user_new_password}
        )
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
        log("[10/43] user forbidden from admin endpoints")
        admin_list_as_user = self.api.admin_list_users(user_token3)
        expect_status(admin_list_as_user, 403, "GET /admin/users (as non-admin)")

        return TestContext(
            admin_token=admin_token,
            user_token=user_token3,
            user_id=user_id,
            user_email=user_email,
        )

    def test_project_management(self, ctx: TestContext) -> None:
        # 11) smoke-test user makes new project
        log("[11/43] user creates project")
        create_proj = self.api.create_project(
            ctx.user_token, "My Project v1", "My Description"
        )
        expect_status(create_proj, 201, "POST /projects")
        proj_data = create_proj.json()
        if not proj_data.get("id"):
            raise TestFailure("Create project did not return an id")
        project1 = Project(
            id=proj_data["id"],
            name=proj_data["name"],
            description=proj_data.get("description"),
            created_at=proj_data.get("created_at"),
            updated_at=proj_data.get("updated_at"),
        )

        # 12) smoke-test user gets project
        log("[12/43] user gets project")
        get_proj = self.api.get_project(ctx.user_token, project1.id)
        expect_status(get_proj, 200, "GET /projects/{id}")
        self.vlog(f"get project response: {get_proj.json()}")

        # 13) smoke-test user lists all project
        log("[13/43] user lists projects")
        list_projs = self.api.list_projects(ctx.user_token)
        expect_status(list_projs, 200, "GET /projects")
        projs = list_projs.json()
        if not any(p["id"] == project1.id for p in projs):
            raise TestFailure(f"Project {project1.id} not found in project list")

        # 14) smoke-test user updates project
        log("[14/43] user updates project")
        update_proj = self.api.update_project(
            ctx.user_token, project1.id, {"name": "Updated Project"}
        )
        expect_status_in(update_proj, (200, 204), "PATCH /projects/{id}")

        # 15) smoke-test user deletes project
        log("[15/43] user deletes project")
        delete_proj = self.api.delete_project(ctx.user_token, project1.id)
        expect_status_in(delete_proj, (200, 204), "DELETE /projects/{id}")

        # 16) smoke-test user gets project -> must fail
        log("[16/43] user gets deleted project (must fail)")
        get_deleted_proj = self.api.get_project(ctx.user_token, project1.id)
        expect_status(get_deleted_proj, 404, "GET /projects/{id} (deleted)")

        # 17) smoke-test user makes new project
        log("[17/43] user creates another project")
        create_proj2 = self.api.create_project(ctx.user_token, "Admin Test Project")
        expect_status(create_proj2, 201, "POST /projects")
        proj_data2 = create_proj2.json()
        project2 = Project(
            id=proj_data2["id"],
            name=proj_data2["name"],
            description=proj_data2.get("description"),
            created_at=proj_data2.get("created_at"),
            updated_at=proj_data2.get("updated_at"),
        )

        log("[18/43] admin lists user's projects")
        admin_list_projs = self.api.admin_list_user_projects(
            ctx.admin_token, ctx.user_id
        )
        expect_status(admin_list_projs, 200, "GET /admin/users/{userId}/projects")
        admin_projs = admin_list_projs.json()
        if not any(p["id"] == project2.id for p in admin_projs):
            raise TestFailure(
                f"Project {project2.id} not found in admin list of user projects"
            )

        # 19) admin gets the project
        log("[19/43] admin gets user's project")
        admin_get_proj = self.api.admin_get_user_project(
            ctx.admin_token, ctx.user_id, project2.id
        )
        expect_status(
            admin_get_proj, 200, "GET /admin/users/{userId}/projects/{projectId}"
        )

        # 20) admin updates project
        log("[20/43] admin updates user's project")
        admin_update_proj = self.api.admin_update_user_project(
            ctx.admin_token, ctx.user_id, project2.id, {"description": "Admin updated this"}
        )
        expect_status_in(
            admin_update_proj, (200, 204), "PATCH /admin/users/{userId}/projects/{projectId}"
        )

        # 21) admin deletes project
        log("[21/43] admin deletes user's project")
        admin_delete_proj = self.api.admin_delete_user_project(
            ctx.admin_token, ctx.user_id, project2.id
        )
        expect_status_in(
            admin_delete_proj, (200, 204), "DELETE /admin/users/{userId}/projects/{projectId}"
        )

        # 22) admin gets project -> must fail
        log("[22/43] admin gets deleted project (must fail)")
        admin_get_deleted_proj = self.api.admin_get_user_project(
            ctx.admin_token, ctx.user_id, project2.id
        )
        expect_status(
            admin_get_deleted_proj,
            404,
            "GET /admin/users/{userId}/projects/{projectId} (deleted)",
        )

    def test_task_management(self, ctx: TestContext) -> None:
        # 23) User creates Project
        log("[23/43] user creates project for task tests")
        create_p3 = self.api.create_project(ctx.user_token, "Task Test Project")
        expect_status(create_p3, 201, "POST /projects")
        project3 = Project(id=create_p3.json()["id"], name="Task Test Project")

        # 24) User creates task in project
        log("[24/43] user creates task in project")
        create_t = self.api.create_task(
            ctx.user_token, project3.id, "My Task", "My Task Desc"
        )
        expect_status(create_t, 201, "POST /projects/{projectId}/tasks")
        t_data = create_t.json()
        task1 = Task(
            id=t_data["id"],
            title=t_data["title"],
            description=t_data.get("description"),
            project_id=project3.id,
        )

        # 25) User lists tasks
        log("[25/43] user lists tasks in project")
        list_t = self.api.list_project_tasks(ctx.user_token, project3.id)
        expect_status(list_t, 200, "GET /projects/{projectId}/tasks")
        tasks = list_t.json()
        if not any(t["id"] == task1.id for t in tasks):
            raise TestFailure(f"Task {task1.id} not found in task list")

        # 26) User gets task he created
        log("[26/43] user gets task")
        get_t = self.api.get_task(ctx.user_token, task1.id)
        expect_status(get_t, 200, "GET /tasks/{id}")

        # 27) User updates task title & description
        log("[27/43] user updates task title & description")
        update_t = self.api.update_task(
            ctx.user_token,
            task1.id,
            {"title": "Updated Task Title", "description": "Updated Task Desc"},
        )
        expect_status_in(update_t, (200, 204), "PATCH /tasks/{id} (title/desc)")

        # 28) User updates task as complete
        log("[28/43] user updates task as complete")
        complete_t = self.api.update_task(ctx.user_token, task1.id, {"is_completed": True})
        expect_status_in(complete_t, (200, 204), "PATCH /tasks/{id} (complete)")

        # 29) User updates task as incomplete
        log("[29/43] user updates task as incomplete")
        incomplete_t = self.api.update_task(
            ctx.user_token, task1.id, {"is_completed": False}
        )
        expect_status_in(incomplete_t, (200, 204), "PATCH /tasks/{id} (incomplete)")

        # 30) User deletes Task
        log("[30/43] user deletes task")
        del_t = self.api.delete_task(ctx.user_token, task1.id)
        expect_status_in(del_t, (200, 204), "DELETE /tasks/{id}")

        # 31) Admin restores Task
        log("[31/43] admin restores task")
        restore_t = self.api.admin_restore_user_task(
            ctx.admin_token, ctx.user_id, task1.id
        )
        expect_status_in(
            restore_t, (200, 204), "POST /admin/users/{userId}/tasks/{taskId}/restore"
        )

        # 32) User creates tag
        log("[32/43] user creates tag")
        tag_resp = self.api.create_tag(ctx.user_token, "Tag 1")
        expect_status(tag_resp, 201, "POST /tags")
        tag1_id = tag_resp.json()["id"]

        # 33) User adds tag to task
        log("[33/43] user adds tag to task")
        add_tag_resp = self.api.update_task_tags(ctx.user_token, task1.id, [tag1_id])
        expect_status(add_tag_resp, 200, "PUT /tasks/{id}/tags")

        # 34) User updates tag
        log("[34/43] user updates tag")
        upd_tag_resp = self.api.update_tag(ctx.user_token, tag1_id, "Tag 1 Updated")
        expect_status_in(upd_tag_resp, (200, 204), "PATCH /tags/{id}")

        # 35) User deletes tag
        log("[35/43] user deletes tag")
        del_tag_resp = self.api.delete_tag(ctx.user_token, tag1_id)
        expect_status_in(del_tag_resp, (200, 204), "DELETE /tags/{id}")

        # 36) User adds tag
        log("[36/43] user adds tag")
        tag2_resp = self.api.create_tag(ctx.user_token, "Tag 2")
        expect_status(tag2_resp, 201, "POST /tags")
        tag2_id = tag2_resp.json()["id"]

        # 37) Admin lists tags of user
        log("[37/43] admin lists tags of user")
        admin_tags_resp = self.api.admin_list_user_tags(ctx.admin_token, ctx.user_id)
        expect_status(admin_tags_resp, 200, "GET /admin/users/{id}/tags")
        admin_tags = admin_tags_resp.json()
        if not any(t["id"] == tag2_id for t in admin_tags):
            raise TestFailure(f"Tag {tag2_id} not found in admin tag list")

        # 38) Admin deletes tag
        log("[38/43] admin deletes tag")
        admin_del_tag = self.api.admin_delete_user_tag(ctx.admin_token, ctx.user_id, tag2_id)
        expect_status_in(admin_del_tag, (200, 204), "DELETE /admin/users/{uId}/tags/{tId}")

        # 39) Admin deletes Task
        log("[39/43] admin deletes task")
        admin_del_t = self.api.admin_delete_user_task(
            ctx.admin_token, ctx.user_id, task1.id
        )
        expect_status_in(
            admin_del_t, (200, 204), "DELETE /admin/users/{userId}/tasks/{taskId}"
        )

        # 40) User deletes project
        log("[40/43] user deletes project")
        del_p3 = self.api.delete_project(ctx.user_token, project3.id)
        expect_status_in(del_p3, (200, 204), "DELETE /projects/{id}")

        # 41) Admin restores project
        log("[41/43] admin restores project")
        restore_p3 = self.api.admin_restore_user_project(
            ctx.admin_token, ctx.user_id, project3.id
        )
        expect_status_in(
            restore_p3,
            (200, 204),
            "POST /admin/users/{userId}/projects/{projectId}/restore",
        )

        # 42) Admin deletes project
        log("[42/43] admin deletes project")
        admin_del_p3 = self.api.admin_delete_user_project(
            ctx.admin_token, ctx.user_id, project3.id
        )
        expect_status_in(
            admin_del_p3,
            (200, 204),
            "DELETE /admin/users/{userId}/projects/{projectId}",
        )

    def test_recurrence(self, ctx: TestContext) -> None:
        import datetime
        # 43) Recurrence lifecycle
        log("[43/43] recurrence lifecycle")

        # Create a project for recurrence tests
        resp = self.api.create_project(ctx.user_token, "Recurrence Project")
        expect_status(resp, 201, "POST /projects (recurrence)")
        project_id = resp.json()["id"]

        # 1. Create a daily recurring task
        now = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0)
        due_at = now.strftime("%Y-%m-%dT%H:%M:%SZ")

        resp = self.api.create_task(ctx.user_token, project_id, "Daily Task",
                                   due_at=due_at, repeat_every=1, repeat_unit="day")
        expect_status(resp, 201, "POST /projects/{id}/tasks (daily recurring)")
        task = resp.json()
        task_id = task["id"]

        # 2. List occurrences for the next 7 days
        to_time = (now + datetime.timedelta(days=7)).strftime("%Y-%m-%dT%H:%M:%SZ")
        resp = self.api.list_occurrences(ctx.user_token, task_id, to_time=to_time)
        expect_status(resp, 200, "GET /tasks/{id}/occurrences")
        occurrences = resp.json()
        if len(occurrences) < 7:
            raise TestFailure(f"Expected at least 7 occurrences, got {len(occurrences)}")

        # 3. Complete the first occurrence
        occ0 = occurrences[0]
        resp = self.api.update_occurrence(ctx.user_token, task_id, occ0["id"], True)
        expect_status(resp, 200, "PATCH /tasks/{id}/occurrences/{occId}")
        if resp.json().get("completed_at") is None:
            raise TestFailure("Occurrence completed_at should not be null after update")

        # 4. Check if task.next_due_at updated
        resp = self.api.get_task(ctx.user_token, task_id)
        expect_status(resp, 200, "GET /tasks/{id}")
        updated_task = resp.json()
        if not updated_task.get("next_due_at"):
            raise TestFailure("Task next_due_at should be set for recurring task")
        if updated_task["next_due_at"] == occ0["due_at"]:
             raise TestFailure("Task next_due_at should have updated after completing occurrence")

        log("✅ Recurrence tests passed.")


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
