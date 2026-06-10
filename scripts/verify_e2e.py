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

# --- 9: Corporate-account groups ---
bob_email = f"bob_{int(time.time())}@test.ru"
print("9) groups / corporate account")
st, bob_reg = req("POST", "/v0/auth/register", jbody={"email": bob_email, "password": "password123", "full_name": "Bob"})
ok("register bob 201", st == 201, st)
bob_token = bob_reg["access_token"]

# Alice creates a group.
st, grp = req("POST", "/v0/groups", token, jbody={"name": "Acme Corp", "description": "shared workspace"})
ok("create group 201", st == 201, st)
gid_grp = grp["id"] if grp else None
ok("group has id", bool(gid_grp), gid_grp)
ok("group role=corporate", grp.get("role") == "corporate", grp.get("role"))

# Alice adds Bob by email.
st, member = req("POST", f"/v0/groups/{gid_grp}/members", token, jbody={"email": bob_email})
ok("add member 201", st == 201, st)
ok("member user_id present", bool(member and member.get("user_id")), member)

# Group detail shows both members.
st, detail = req("GET", f"/v0/groups/{gid_grp}", token)
ok("get group 200", st == 200, st)
ok("group has 2 members", len(detail.get("members", [])) == 2, len(detail.get("members", [])))

# Bob (different account, same group) can see Alice's document.
st, bob_docs = req("GET", "/v0/documents", bob_token)
ok("bob sees shared docs (group access)", st == 200 and bob_docs["meta"]["total"] >= 1, bob_docs["meta"]["total"] if bob_docs else 0)

# Bob can see Alice's plan in his plan list.
st, bob_plans = req("GET", "/v0/plans", bob_token)
ok("bob sees shared plans", st == 200 and bob_plans["meta"]["total"] >= 1, bob_plans["meta"]["total"] if bob_plans else 0)

# Non-member (fresh outsider account) gets 403 on the group and 0 shared docs.
eve_email = f"eve_{int(time.time())}@outside.ru"
st, eve_reg = req("POST", "/v0/auth/register", jbody={"email": eve_email, "password": "password123", "full_name": "Eve"})
eve_token = eve_reg["access_token"]
st, _ = req("GET", f"/v0/groups/{gid_grp}", eve_token)
ok("eve cannot access group (403)", st == 403, st)
st, eve_docs = req("GET", "/v0/documents", eve_token)
ok("eve sees no shared docs", st == 200 and eve_docs["meta"]["total"] == 0, eve_docs["meta"]["total"] if eve_docs else "err")

# Creator can't remove themselves.
st, _ = req("DELETE", f"/v0/groups/{gid_grp}/members/1", token)
# (user IDs are sequential; Alice is id=1 if first registered in this run — use member id from detail)
alice_uid = next((m["user_id"] for m in detail.get("members", []) if m.get("email") == email), None)
if alice_uid:
    st, _ = req("DELETE", f"/v0/groups/{gid_grp}/members/{alice_uid}", token)
    ok("creator cannot remove self (409)", st == 409, st)

# Bob's member_id
bob_uid = member.get("user_id") if member else None
if bob_uid:
    st, _ = req("DELETE", f"/v0/groups/{gid_grp}/members/{bob_uid}", token)
    ok("remove bob 204", st == 204, st)
    # After removal Bob should see 0 shared docs.
    st, bob_docs2 = req("GET", "/v0/documents", bob_token)
    ok("bob sees 0 docs after removal from group", st == 200 and bob_docs2["meta"]["total"] == 0, bob_docs2["meta"]["total"] if bob_docs2 else "err")

print("\nDONE")
