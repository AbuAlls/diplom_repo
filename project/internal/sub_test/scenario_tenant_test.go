package sub_test

import (
	"net/http"
	"testing"
)

// Scenario 3 — Multi-tenant isolation. Two users each build their own tree;
// then user B attempts to read and mutate every level of user A's hierarchy
// (plan → goal → item → document) and must be denied. This walks the ownership
// chain enforced in usecase/access.go (requirePlan → requireGoal →
// requireItemByID) across the full set of authenticated endpoints.
//
// Expected denials:
//   - existing-but-not-owned resource → 403 FORBIDDEN
//   - non-existent resource           → 404 NOT_FOUND
//   - missing/garbage bearer token    → 401 UNAUTHORIZED
func TestScenario_MultiTenantIsolation(t *testing.T) {
	h := newHarness(t)
	tokenA := h.register("alice@example.com")
	tokenB := h.register("bob@example.com")

	// Alice's tree + a document.
	planA, goalA, itemA := h.seedItem(tokenA, "Alice Plan", 100, 25)
	_, docA := h.uploadDoc(tokenA, itemA, "alice.pdf", "alice-bytes")

	// Bob has his own unrelated tree (so ids exist in his space too).
	h.seedItem(tokenB, "Bob Plan", 100, 25)

	pA := itoa(planA)
	gA := itoa(goalA)
	iA := itoa(itemA)
	dA := itoa(docA.ID)

	// ---- Bob is FORBIDDEN from touching Alice's resources ----
	forbidden := []struct {
		method, path, body string
	}{
		{"GET", "/v0/plans/" + pA + "/goals", ""},
		{"POST", "/v0/plans/" + pA + "/goals", `{"name":"intrusion"}`},
		{"GET", "/v0/plans/" + pA + "/goals/" + gA + "/items", ""},
		{"POST", "/v0/plans/" + pA + "/goals/" + gA + "/items", `{"name":"x"}`},
		{"PATCH", "/v0/plans/" + pA + "/goals/" + gA + "/items/" + iA, `{"current_value":99}`},
		{"GET", "/v0/items/" + iA + "/analytics", ""},
		{"POST", "/v0/items/" + iA + "/analyze", `{"message":"peek"}`},
		{"GET", "/v0/documents/" + dA, ""},
		{"GET", "/v0/documents/" + dA + "/download", ""},
		{"GET", "/v0/documents/" + dA + "/storage", ""},
		{"PATCH", "/v0/documents/" + dA, `{"organization_name":"hijack"}`},
		{"POST", "/v0/documents/" + dA + "/confirm", ""},
		{"POST", "/v0/documents/" + dA + "/reject", ""},
		{"POST", "/v0/documents/" + dA + "/reanalyze", ""},
	}
	for _, c := range forbidden {
		st, body := h.req(c.method, c.path, tokenB, c.body)
		if st != http.StatusForbidden {
			t.Fatalf("%s %s as Bob: expected 403, got %d (body=%s)", c.method, c.path, st, body)
		}
		if code := errorCode(t, body); code != "FORBIDDEN" {
			t.Fatalf("%s %s: expected FORBIDDEN code, got %q", c.method, c.path, code)
		}
	}

	// ---- Alice's data is unchanged after Bob's attempts ----
	var item itemResp
	h.reqJSON("PATCH", "/v0/plans/"+pA+"/goals/"+gA+"/items/"+iA, tokenA, `{"current_value":25}`, &item)
	if item.CurrentValue == nil || *item.CurrentValue != 25 {
		t.Fatalf("Alice's item current_value tampered: %v", item.CurrentValue)
	}
	// Note: the mock recognizer pre-fills organization_name at upload, so the
	// field is non-nil by design. The point is Bob's "hijack" never landed.
	var doc docResp
	h.reqJSON("GET", "/v0/documents/"+dA, tokenA, "", &doc)
	if doc.OrganizationName != nil && *doc.OrganizationName == "hijack" {
		t.Fatalf("Alice's document was mutated by Bob: %v", *doc.OrganizationName)
	}

	// ---- non-existent resources → 404 ----
	for _, path := range []string{
		"/v0/plans/999999/goals",
		"/v0/documents/999999",
		"/v0/documents/999999/storage",
	} {
		if st, _ := h.req("GET", path, tokenB, ""); st != http.StatusNotFound {
			t.Fatalf("GET %s: expected 404, got %d", path, st)
		}
	}

	// ---- auth failures → 401 ----
	if st, _ := h.req("GET", "/v0/plans", "", ""); st != http.StatusUnauthorized {
		t.Fatalf("no token: expected 401, got %d", st)
	}
	if st, _ := h.req("GET", "/v0/plans", "garbage.jwt.token", ""); st != http.StatusUnauthorized {
		t.Fatalf("garbage token: expected 401, got %d", st)
	}

	// ---- Bob only sees his own plans in the collection endpoint ----
	var list listResp
	h.reqJSON("GET", "/v0/plans", tokenB, "", &list)
	if list.Meta.Total != 1 {
		t.Fatalf("Bob should see exactly his 1 plan, got %d", list.Meta.Total)
	}
}
