#!/usr/bin/env python3
"""Сквозная проверка маппинга фронтенда на бэкенд (использует те же эндпоинты, что api.js)."""
import json, time, urllib.request, urllib.parse, uuid

B = "http://localhost:8080"

def req(method, path, token=None, jbody=None, form=None, files=None):
    url = B + path
    headers = {}
    data = None
    if token:
        headers["Authorization"] = "Bearer " + token
    if jbody is not None:
        data = json.dumps(jbody).encode()
        headers["Content-Type"] = "application/json"
    elif form is not None:
        data = urllib.parse.urlencode(form).encode()
        headers["Content-Type"] = "application/x-www-form-urlencoded"
    elif files is not None:
        boundary = "----b" + uuid.uuid4().hex
        parts = []
        for name, (fn, content) in files.items():
            parts.append(("--" + boundary).encode())
            parts.append(f'Content-Disposition: form-data; name="{name}"; filename="{fn}"'.encode())
            parts.append(b"Content-Type: text/plain")
            parts.append(b"")
            parts.append(content)
        parts.append(("--" + boundary + "--").encode())
        data = b"\r\n".join(parts)
        headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
    r = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r) as resp:
            body = resp.read().decode()
            return resp.status, (json.loads(body) if body else None)
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read().decode() or "null")

ok = lambda label, cond, extra="": print(("  ✅" if cond else "  ❌") + f" {label} {extra}")

email = f"e2e_{int(time.time())}@test.ru"
print("1) register")
st, reg = req("POST", "/v0/auth/register", jbody={"email": email, "password": "password123", "full_name": "E2E"})
ok("register 201", st == 201, st)
token = reg["access_token"]

print("2) login (form)")
st, lg = req("POST", "/v0/auth/token", form={"username": email, "password": "password123"})
ok("login 200 + token", st == 200 and "access_token" in lg, st)

print("3) plan/goal/item")
st, plan = req("POST", "/v0/plans", token, jbody={"name": "План", "status": "active"})
ok("create plan", st == 201, st); pid = plan["id"]
st, goal = req("POST", f"/v0/plans/{pid}/goals", token, jbody={"name": "Цель", "sort_order": 0})
ok("create goal", st == 201, st); gid = goal["id"]
st, item = req("POST", f"/v0/plans/{pid}/goals/{gid}/items", token,
               jbody={"name": "Позиция", "target_value": 100, "current_value": 40, "unit": "шт"})
ok("create item", st == 201, st); iid = item["id"]

print("4) items list -> progress_percent")
st, its = req("GET", f"/v0/plans/{pid}/goals/{gid}/items", token)
pp = its["items"][0].get("progress_percent")
ok("progress_percent computed (40/100=40)", pp == 40, pp)

print("5) upload document to item")
st, up = req("POST", f"/v0/documents/upload/{iid}", token,
             files={"file": ("invoice.txt", "INVOICE 2026\nОрганизация: ООО Тест\nИНН 7700000000\n".encode())})
ok("upload 200/201", st in (200, 201), st)
ok("doc has status", bool(up and up.get("status")), up.get("status") if up else None)
did = up["id"]

print("6) documents list (what DocumentsScreen renders)")
st, docs = req("GET", "/v0/documents", token)
ok("list has uploaded doc", any(d["id"] == did for d in docs["items"]), f"total={docs['meta']['total']}")

print("7) get document (what DocumentDetail fetches)")
st, doc = req("GET", f"/v0/documents/{did}", token)
ok("get 200", st == 200, st)
print("    fields:", {k: doc.get(k) for k in ["title", "status", "organization_name", "inn", "recognized_text", "file_name", "mime_type"]})

print("8) item analytics")
st, an = req("GET", f"/v0/items/{iid}/analytics", token)
ok("analytics 200", st == 200, st)
print("    source_documents_count:", an.get("source_documents_count"), "progress:", an.get("progress_percent"))

print("\nDONE")
