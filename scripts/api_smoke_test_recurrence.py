import json
import time
import unittest
import urllib.request
import urllib.error
import datetime
from typing import Optional, Any

# Re-using logic from the main smoke test to keep it standard-lib only
class HttpClient:
    def __init__(self, base_url: str, timeout_sec: float = 10.0):
        self.base_url = base_url.rstrip("/")
        self.timeout_sec = timeout_sec

    def request(self, method: str, path: str, token: Optional[str] = None, json_body: Optional[dict[str, Any]] = None) -> 'HTTPResponse':
        url = f"{self.base_url}/{path.lstrip('/')}"
        headers = {}
        if token:
            headers["Authorization"] = f"Bearer {token}"
        
        data = None
        if json_body is not None:
            data = json.dumps(json_body).encode("utf-8")
            headers["Content-Type"] = "application/json"

        req = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout_sec) as resp:
                return HTTPResponse(resp.status, resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            return HTTPResponse(e.code, e.read().decode("utf-8"))
        except Exception as e:
            return HTTPResponse(0, str(e))

class HTTPResponse:
    def __init__(self, status: int, body: str):
        self.status = status
        self.body = body

    def json(self) -> Any:
        try:
            return json.loads(self.body)
        except:
            return None

class GoToDoAPI:
    def __init__(self, base_url: str):
        self.client = HttpClient(base_url)

    def signup(self, email: str, password: str):
        return self.client.request("POST", "/api/v1/auth/signup", json_body={"email": email, "password": password})

    def login(self, email: str, password: str):
        return self.client.request("POST", "/api/v1/auth/login", json_body={"email": email, "password": password})

    def create_project(self, token: str, name: str):
        return self.client.request("POST", "/api/v1/projects", token=token, json_body={"name": name})

    def create_task(self, token: str, project_id: int, title: str, **kwargs):
        body = {"title": title}
        body.update(kwargs)
        return self.client.request("POST", f"/api/v1/projects/{project_id}/tasks", token=token, json_body=body)

    def list_occurrences(self, token: str, task_id: int, from_time: Optional[str] = None, to_time: Optional[str] = None):
        path = f"/api/v1/tasks/{task_id}/occurrences"
        params = []
        if from_time: params.append(f"from={from_time}")
        if to_time: params.append(f"to={to_time}")
        if params:
            path += "?" + "&".join(params)
        return self.client.request("GET", path, token=token)

    def update_occurrence(self, token: str, task_id: int, occurrence_id: int, completed: bool):
        return self.client.request("PATCH", f"/api/v1/tasks/{task_id}/occurrences/{occurrence_id}", token=token, json_body={"completed": completed})

    def get_task(self, token: str, task_id: int):
        return self.client.request("GET", f"/api/v1/tasks/{task_id}", token=token)

class TestRecurrenceAPI(unittest.TestCase):
    base_url = "http://localhost:8080"
    
    @classmethod
    def setUpClass(cls):
        cls.api = GoToDoAPI(cls.base_url)
        # Create a test user
        ts = int(time.time())
        cls.email = f"recurrence_test_{ts}@example.com"
        cls.password = "Password123!"
        resp = cls.api.signup(cls.email, cls.password)
        if resp.status not in (200, 201):
             # Maybe already exists if we retry fast, try login
             pass
        
        resp = cls.api.login(cls.email, cls.password)
        if resp.status != 200:
            raise Exception(f"Failed to login: {resp.body}")
        cls.token = resp.json()["token"]
        
        # Create a project
        resp = cls.api.create_project(cls.token, "Recurrence Project")
        if resp.status != 201:
            raise Exception(f"Failed to create project: {resp.body}")
        cls.project_id = resp.json()["id"]

    def test_daily_recurrence_lifecycle(self):
        # 1. Create a daily recurring task
        # Anchor it today
        now = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0)
        due_at = now.strftime("%Y-%m-%dT%H:%M:%SZ")
        
        resp = self.api.create_task(self.token, self.project_id, "Daily Task", 
                                   due_at=due_at, repeat_every=1, repeat_unit="day")
        self.assertEqual(resp.status, 201, resp.body)
        task = resp.json()
        task_id = task["id"]
        
        # 2. List occurrences for the next 7 days
        to_time = (now + datetime.timedelta(days=7)).strftime("%Y-%m-%dT%H:%M:%SZ")
        resp = self.api.list_occurrences(self.token, task_id, to_time=to_time)
        self.assertEqual(resp.status, 200, resp.body)
        occurrences = resp.json()
        
        # Should have around 8 occurrences (today + 7 days)
        self.assertGreaterEqual(len(occurrences), 7)
        
        # 3. Complete the first occurrence
        occ0 = occurrences[0]
        resp = self.api.update_occurrence(self.token, task_id, occ0["id"], True)
        self.assertEqual(resp.status, 200, resp.body)
        self.assertIsNotNone(resp.json()["completed_at"])
        
        # 4. Check if task.next_due_at updated
        resp = self.api.get_task(self.token, task_id)
        self.assertEqual(resp.status, 200, resp.body)
        updated_task = resp.json()
        self.assertIsNotNone(updated_task["next_due_at"])
        # next_due_at should be > due_at of completed occurrence
        self.assertNotEqual(updated_task["next_due_at"], occ0["due_at"])

    def test_invalid_recurrence_params(self):
        # Missing repeat_unit
        resp = self.api.create_task(self.token, self.project_id, "Invalid Task", repeat_every=1)
        self.assertEqual(resp.status, 400)
        
        # Invalid repeat_unit
        due_at = datetime.datetime.now(datetime.timezone.utc).isoformat()
        resp = self.api.create_task(self.token, self.project_id, "Invalid Unit", 
                                   due_at=due_at, repeat_every=1, repeat_unit="decade")
        # The backend might accept it and then fail during occurrence generation
        # or have a list of valid units. occurrences.go shows day, week, month.
        # Let's see what the handler does.
        
    def test_non_recurring_task_occurrences(self):
        resp = self.api.create_task(self.token, self.project_id, "Simple Task")
        self.assertEqual(resp.status, 201)
        task_id = resp.json()["id"]
        
        resp = self.api.list_occurrences(self.token, task_id)
        # Should be 404 because recurringTaskVisible checks for repeat_every/unit
        self.assertEqual(resp.status, 404)

    def test_unauthorized_access(self):
        # Create a task for user A
        due_at = datetime.datetime.now(datetime.timezone.utc).replace(microsecond=0).strftime("%Y-%m-%dT%H:%M:%SZ")
        resp = self.api.create_task(self.token, self.project_id, "Secret Task", 
                                   due_at=due_at, repeat_every=1, repeat_unit="day")
        task_id = resp.json()["id"]
        
        # Create user B
        email_b = f"user_b_{int(time.time())}@example.com"
        self.api.signup(email_b, "Password123!")
        resp = self.api.login(email_b, "Password123!")
        token_b = resp.json()["token"]
        
        # User B tries to list occurrences of User A's task
        resp = self.api.list_occurrences(token_b, task_id)
        self.assertEqual(resp.status, 404)

if __name__ == "__main__":
    import sys
    # Allow overriding base_url via args if needed
    if len(sys.argv) > 1 and sys.argv[1].startswith("http"):
        TestRecurrenceAPI.base_url = sys.argv[1]
        # Remove it so unittest doesn't complain
        sys.argv.pop(1)
        
    unittest.main()
