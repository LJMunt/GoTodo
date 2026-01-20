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


class SmokeTestRunner:
    def __init__(self, api: GoToDoAPI, verbose: bool = False) -> None:
        self.api = api
        self.verbose = verbose

    def log(self, msg: str) -> None:
        print(msg)

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
        self.log("== GoToDo API smoke test ==")
        self.log("Waiting for API readiness...")
        self.api.wait_ready(timeout_sec=10.0)


    # 1) Health/ready
        self.log("[1/10] health + ready")
        expect_status(self.api.health(), 200, "GET /health")
        expect_status(self.api.ready(), 200, "GET /ready")

        # 2) Login admin (admin must already exist and be admin in DB)
        self.log("[2/10] admin login")
        admin_login = self.api.login(admin_email, admin_password)
        expect_status(admin_login, 200, "POST /auth/login (admin)")
        admin_token = admin_login.json().get("token")
        if not admin_token:
            raise TestFailure("Admin login did not return a token")

        # 3) Create unique user
        uniq = f"{_now_ms()}-{uuid.uuid4().hex[:10]}"
        user_email = f"{user_email_prefix}+{uniq}@example.com" if "@" not in user_email_prefix else user_email_prefix.replace("@", f"+{uniq}@")
        self.log(f"[3/10] signup user: {user_email}")
        signup = self.api.signup(user_email, user_password)
        # Depending on your signup handler, you may return 201 with user json (and possibly a token).
        expect_status(signup, 201, "POST /auth/signup (user)")

        # 4) Login as the new user
        self.log("[4/10] user login")
        user_login = self.api.login(user_email, user_password)
        expect_status(user_login, 200, "POST /auth/login (user)")
        user_token = user_login.json().get("token")
        if not user_token:
            raise TestFailure("User login did not return a token")

        # 5) /me should work and return id
        self.log("[5/10] GET /users/me")
        me = self.api.me(user_token)
        expect_status(me, 200, "GET /users/me")
        me_json = me.json()
        self.vlog(f"/me response: {me_json}")
        user_id = me_json.get("id")
        if not isinstance(user_id, int):
            raise TestFailure(f"/me did not return integer id. Got: {user_id}")

        # 6) Admin disables user (soft delete)
        self.log("[6/10] admin disables user (soft delete)")
        disable = self.api.admin_disable_user(admin_token, user_id)
        expect_status(disable, 204, "DELETE /admin/users/{id}")

        # 7) Disabled user should fail login (your login returns 401 for invalid/disabled)
        self.log("[7/10] disabled user cannot login")
        disabled_login = self.api.login(user_email, user_password)
        expect_status(disabled_login, 401, "POST /auth/login (disabled user)")

        # 8) Admin re-enables user
        self.log("[8/10] admin re-enables user")
        enable = self.api.admin_update_user(admin_token, user_id, {"is_active": True})
        expect_status_in(enable, (200, 204), "PATCH /admin/users/{id} (enable)")
        self.vlog(f"enable response: {enable.json()}")

        # 9) User can login again, then admin changes password, then user can login with new password
        self.log("[9/10] password change cycle")
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
        self.log("[10/10] user forbidden from admin endpoints")
        admin_list_as_user = self.api.admin_list_users(user_token3)
        expect_status(admin_list_as_user, 403, "GET /admin/users (as non-admin)")

        self.log("✅ All smoke tests passed.")


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
