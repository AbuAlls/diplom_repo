package sub_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Scenario 4 — AI analytics agent callback loop and its security envelope.
//
// When a user calls POST /v0/items/{id}/analyze, AnalyticsService mints a
// short-lived session nonce (shared SessionStore) and injects it into the
// message. The scripted analyzer plays the external agent: it extracts the
// nonce and calls GET /api/schema and POST /api/analytics/query with it. This
// test asserts the whole chain plus its guarantees:
//
//   - the nonce resolves to the initiating user (per-tenant row scoping);
//   - the nonce is single-use within the call — it is deleted once analyze
//     returns, so reusing it afterwards yields 401;
//   - the global INTERNAL_API_TOKEN still works but is UNSCOPED (sees all rows);
//   - the read-only SQL guard rejects mutations and statement chaining;
//   - bad/empty internal tokens are rejected with 401.
func TestScenario_AIAgentCallbackLoop(t *testing.T) {
	h := newHarness(t)
	const emailA, emailB = "owner-a@example.com", "owner-b@example.com"
	tokenA := h.register(emailA)
	tokenB := h.register(emailB)
	_, _, itemA := h.seedItem(tokenA, "A plan", 100, 10)
	_, _, itemB := h.seedItem(tokenB, "B plan", 100, 10)

	ownerA := h.userIDFor(emailA)
	ownerB := h.userIDFor(emailB)

	// Seed the analytics query repo with rows for BOTH owners. Scoping must
	// ensure A's agent only sees A's rows.
	h.users.query.seed(ownerA, 101, "A-invoice-1")
	h.users.query.seed(ownerA, 102, "A-invoice-2")
	h.users.query.seed(ownerB, 201, "B-invoice-1")

	// --- A runs the agent: should observe exactly A's two rows ---
	var ar analyzeResp
	if st := h.reqJSON("POST", "/v0/items/"+itoa(itemA)+"/analyze", tokenA, `{"message":"audit me","model":"m1"}`, &ar); st != http.StatusOK {
		t.Fatalf("A analyze: status %d", st)
	}
	az := h.users.analyzer
	if az.httpErr != nil {
		t.Fatalf("A callback loop failed: %v", az.httpErr)
	}
	if ar.Recommendations != "analyzed 2 documents" {
		t.Fatalf("A: expected 2 scoped rows, got %q", ar.Recommendations)
	}
	for _, row := range az.rowsSeen {
		if len(row) != 3 {
			t.Fatalf("unexpected row shape: %+v", row)
		}
		// row = [id, title, owner_id]; owner_id decodes from JSON as float64.
		if ownerOf, ok := row[2].(float64); !ok || int64(ownerOf) != ownerA {
			t.Fatalf("A leaked another tenant's row: %+v (want owner %d)", row, ownerA)
		}
	}

	// The nonce A's agent used must now be invalid (deleted on analyze return).
	usedNonce := az.lastNonce
	if usedNonce == "" {
		t.Fatalf("analyzer did not record a nonce")
	}
	if st, _ := h.internalReq("GET", "/api/schema", usedNonce, ""); st != http.StatusUnauthorized {
		t.Fatalf("stale nonce reuse: expected 401, got %d", st)
	}

	// --- B runs the agent: should observe exactly B's single row ---
	if st := h.reqJSON("POST", "/v0/items/"+itoa(itemB)+"/analyze", tokenB, `{"message":"audit me too","model":"m2"}`, &ar); st != http.StatusOK {
		t.Fatalf("B analyze: status %d", st)
	}
	if ar.Recommendations != "analyzed 1 documents" {
		t.Fatalf("B: expected 1 scoped row, got %q", ar.Recommendations)
	}

	// --- direct internal-callback checks (global token path) ---

	// Global token is accepted but UNSCOPED: it sees all three rows.
	var qr queryResp
	st, body := h.internalReq("POST", "/api/analytics/query", internalToken, `{"query":"select id, title, owner_id from documents"}`)
	if st != http.StatusOK {
		t.Fatalf("global-token query: status %d (%s)", st, body)
	}
	decodeInto(t, body, &qr)
	if qr.RowCount != 3 {
		t.Fatalf("global token should see all 3 rows, got %d", qr.RowCount)
	}

	// Schema endpoint via global token.
	if st, _ := h.internalReq("GET", "/api/schema", internalToken, ""); st != http.StatusOK {
		t.Fatalf("global-token schema: status %d", st)
	}

	// --- read-only SQL guard ---
	badQueries := []string{
		`{"query":"delete from documents"}`,
		`{"query":"update documents set title = 'x'"}`,
		`{"query":"drop table documents"}`,
		`{"query":"select 1; select 2"}`, // statement chaining
		`{"query":"insert into documents values (1)"}`,
		`{"query":""}`,
	}
	for _, q := range badQueries {
		st, body := h.internalReq("POST", "/api/analytics/query", internalToken, q)
		if st != http.StatusBadRequest {
			t.Fatalf("guarded query %s: expected 400, got %d", q, st)
		}
		if code := errorCode(t, body); code != "VALIDATION_ERROR" {
			t.Fatalf("guarded query %s: expected VALIDATION_ERROR, got %q", q, code)
		}
	}

	// --- internal auth failures ---
	for _, tok := range []string{"", "definitely-not-valid"} {
		if st, _ := h.internalReq("GET", "/api/schema", tok, ""); st != http.StatusUnauthorized {
			t.Fatalf("internal token %q: expected 401, got %d", tok, st)
		}
		if st, _ := h.internalReq("POST", "/api/analytics/query", tok, `{"query":"select 1"}`); st != http.StatusUnauthorized {
			t.Fatalf("internal token %q query: expected 401, got %d", tok, st)
		}
	}

	// --- analyze input validation ---
	if st, body := h.req("POST", "/v0/items/"+itoa(itemA)+"/analyze", tokenA, `{"message":"   "}`); st != http.StatusBadRequest {
		t.Fatalf("blank message: expected 400, got %d", st)
	} else if code := errorCode(t, body); code != "VALIDATION_ERROR" {
		t.Fatalf("blank message: expected VALIDATION_ERROR, got %q", code)
	}
}

func decodeInto(t *testing.T, body []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("decode: %v; body=%s", err, body)
	}
}
