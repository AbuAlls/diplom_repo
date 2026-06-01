package sub_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Scenario 5 — Pagination, filtering and input validation across the listing
// and creation endpoints. Creates a batch of plans and documents, then pages
// through them and checks the PaginationMeta math (total / total_pages / size
// clamping), the plan_item_id document filter, and validation rejections.
func TestScenario_PaginationAndValidation(t *testing.T) {
	h := newHarness(t)
	token := h.register("paginator@example.com")

	// Create 5 plans.
	const nPlans = 5
	for i := 0; i < nPlans; i++ {
		if st := h.reqJSON("POST", "/v0/plans", token, `{"name":"Plan"}`, nil); st != http.StatusCreated {
			t.Fatalf("create plan %d: status %d", i, st)
		}
	}

	// Page 1, size 2 → 2 items, total 5, total_pages 3.
	var page1 listResp
	h.reqJSON("GET", "/v0/plans?page=1&size=2", token, "", &page1)
	if page1.Meta.Total != nPlans || page1.Meta.TotalPages != 3 || page1.Meta.Size != 2 || page1.Meta.Page != 1 {
		t.Fatalf("page1 meta wrong: %+v", page1.Meta)
	}
	if n := countItems(t, page1.Items); n != 2 {
		t.Fatalf("page1 expected 2 items, got %d", n)
	}

	// Last page (3) has the remainder: 1 item.
	var page3 listResp
	h.reqJSON("GET", "/v0/plans?page=3&size=2", token, "", &page3)
	if n := countItems(t, page3.Items); n != 1 {
		t.Fatalf("page3 expected 1 item, got %d", n)
	}

	// size over the cap (100) is clamped.
	var capped listResp
	h.reqJSON("GET", "/v0/plans?size=9999", token, "", &capped)
	if capped.Meta.Size != 100 {
		t.Fatalf("expected size clamped to 100, got %d", capped.Meta.Size)
	}

	// Bad page/size fall back to defaults (page 1, size 20).
	var defaulted listResp
	h.reqJSON("GET", "/v0/plans?page=-3&size=abc", token, "", &defaulted)
	if defaulted.Meta.Page != 1 || defaulted.Meta.Size != 20 {
		t.Fatalf("expected defaults page=1 size=20, got %+v", defaulted.Meta)
	}

	// ---- document filtering by plan_item_id ----
	_, _, item1 := h.seedItem(token, "Docs Plan A", 10, 1)
	_, _, item2 := h.seedItem(token, "Docs Plan B", 10, 1)
	h.uploadDoc(token, item1, "a1.pdf", "x")
	h.uploadDoc(token, item1, "a2.pdf", "x")
	h.uploadDoc(token, item2, "b1.pdf", "x")

	var item1Docs listResp
	h.reqJSON("GET", "/v0/documents?plan_item_id="+itoa(item1), token, "", &item1Docs)
	if item1Docs.Meta.Total != 2 {
		t.Fatalf("item1 expected 2 docs, got %d", item1Docs.Meta.Total)
	}
	var item2Docs listResp
	h.reqJSON("GET", "/v0/documents?plan_item_id="+itoa(item2), token, "", &item2Docs)
	if item2Docs.Meta.Total != 1 {
		t.Fatalf("item2 expected 1 doc, got %d", item2Docs.Meta.Total)
	}
	// Unfiltered: all 3 of this owner's documents.
	var allDocs listResp
	h.reqJSON("GET", "/v0/documents", token, "", &allDocs)
	if allDocs.Meta.Total != 3 {
		t.Fatalf("expected 3 docs total, got %d", allDocs.Meta.Total)
	}

	// ---- validation rejections on creation ----
	bad := []struct {
		path, body, wantCode string
	}{
		{"/v0/plans", `{"name":""}`, "VALIDATION_ERROR"},
		{"/v0/plans", `{"name":"   "}`, "VALIDATION_ERROR"},
		{"/v0/plans", `not json`, "VALIDATION_ERROR"},
	}
	for _, c := range bad {
		st, body := h.req("POST", c.path, token, c.body)
		if st != http.StatusBadRequest {
			t.Fatalf("POST %s %q: expected 400, got %d", c.path, c.body, st)
		}
		if code := errorCode(t, body); code != c.wantCode {
			t.Fatalf("POST %s %q: expected %s, got %q", c.path, c.body, c.wantCode, code)
		}
	}

	// Registration validation: a password shorter than 8 chars is rejected.
	// NOTE: AuthService.Register surfaces this as ErrInvalidCredentials, so the
	// transport maps it to 401 INVALID_CREDENTIALS (not 400). We assert the
	// service's actual contract rather than the intuitive 400.
	if st, body := h.req("POST", "/v0/auth/register", "", `{"email":"x@y.z","password":"short","full_name":"X"}`); st != http.StatusUnauthorized {
		t.Fatalf("short password: expected 401, got %d", st)
	} else if code := errorCode(t, body); code != "INVALID_CREDENTIALS" {
		t.Fatalf("short password: expected INVALID_CREDENTIALS, got %q", code)
	}
}

// countItems counts elements in the raw JSON "items" array of a list response.
func countItems(t *testing.T, raw json.RawMessage) int {
	t.Helper()
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("items not an array: %v; raw=%s", err, raw)
	}
	return len(arr)
}
