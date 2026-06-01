package sub_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Scenario 1 — Full happy-path lifecycle of a single user, chaining the auth,
// planning, document and analytics subsystems:
//
//	register → login → create plan → create goal → create item
//	→ upload document → patch metadata → confirm
//	→ read item analytics → run the AI agent (which calls back into /api/*).
//
// Touches 11 distinct endpoints and asserts state propagates correctly between
// them (e.g. the confirmed document shows up in the item's analytics counters,
// and the AI agent — scoped by a session nonce — only sees this user's rows).
func TestScenario_FullLifecycle(t *testing.T) {
	h := newHarness(t)
	const email = "founder@example.com"

	// --- auth: register, then exchange credentials via the token endpoint ---
	token := h.register(email)

	st, loginTok := h.login(email, "password123")
	if st != http.StatusOK || loginTok == "" {
		t.Fatalf("login: status=%d token_empty=%v", st, loginTok == "")
	}
	if st, _ := h.login(email, "wrong-password"); st != http.StatusUnauthorized {
		t.Fatalf("login with bad password: expected 401, got %d", st)
	}

	// --- planning tree: plan → goal → item ---
	planID, goalID, itemID := h.seedItem(token, "Procurement 2026", 200, 50)

	// progress should be current/target = 50/200 = 25%.
	var item itemResp
	h.reqJSON("GET", "/v0/plans/"+itoa(planID)+"/goals/"+itoa(goalID)+"/items", token, "", nil)
	if st := h.reqJSON("PATCH", "/v0/plans/"+itoa(planID)+"/goals/"+itoa(goalID)+"/items/"+itoa(itemID), token, `{"current_value":50}`, &item); st != http.StatusOK {
		t.Fatalf("patch item: status %d", st)
	}
	if item.ProgressPercent == nil || *item.ProgressPercent != 25 {
		t.Fatalf("expected progress 25, got %v", item.ProgressPercent)
	}

	// --- documents: upload → patch → confirm ---
	st, doc := h.uploadDoc(token, itemID, "invoice.pdf", "raw invoice bytes")
	if st != http.StatusOK {
		t.Fatalf("upload: status %d", st)
	}
	if doc.Status != "pending_review" {
		t.Fatalf("expected pending_review after upload, got %q", doc.Status)
	}
	if doc.ModelVersion == nil || *doc.ModelVersion != "mock-v0" {
		t.Fatalf("expected mock-v0 model version, got %v", doc.ModelVersion)
	}

	var patched docResp
	if st := h.reqJSON("PATCH", "/v0/documents/"+itoa(doc.ID), token,
		`{"organization_name":"ООО Поставщик","inn":"7701234567","recognized_category_id":3}`, &patched); st != http.StatusOK {
		t.Fatalf("patch document: status %d", st)
	}
	if patched.OrganizationName == nil || *patched.OrganizationName != "ООО Поставщик" {
		t.Fatalf("organization_name not persisted: %v", patched.OrganizationName)
	}
	if patched.RecognizedCategoryID == nil || *patched.RecognizedCategoryID != 3 {
		t.Fatalf("recognized_category_id not persisted: %v", patched.RecognizedCategoryID)
	}

	var confirmed docResp
	if st := h.reqJSON("POST", "/v0/documents/"+itoa(doc.ID)+"/confirm", token, "", &confirmed); st != http.StatusOK {
		t.Fatalf("confirm: status %d", st)
	}
	if confirmed.Status != "confirmed" {
		t.Fatalf("expected confirmed, got %q", confirmed.Status)
	}

	// --- analytics: the confirmed document is now a counted source ---
	var an analyticsResp
	if st := h.reqJSON("GET", "/v0/items/"+itoa(itemID)+"/analytics", token, "", &an); st != http.StatusOK {
		t.Fatalf("analytics: status %d", st)
	}
	if an.SourceDocumentsCount != 1 {
		t.Fatalf("expected 1 source document, got %d", an.SourceDocumentsCount)
	}
	if an.LatestDocumentID == nil || *an.LatestDocumentID != doc.ID {
		t.Fatalf("expected latest_document_id %d, got %v", doc.ID, an.LatestDocumentID)
	}
	if an.Notes == nil {
		t.Fatalf("expected non-null notes array")
	}

	// --- AI agent: scoped read-only access to this user's data ---
	h.users.query.seed(h.userIDFor(email), doc.ID, "invoice.pdf")

	var ar analyzeResp
	if st := h.reqJSON("POST", "/v0/items/"+itoa(itemID)+"/analyze", token, `{"message":"summarise my procurement","model":"test-model"}`, &ar); st != http.StatusOK {
		t.Fatalf("analyze: status %d", st)
	}
	if ar.Recommendations != "analyzed 1 documents" {
		t.Fatalf("unexpected recommendations: %q", ar.Recommendations)
	}

	// The scripted analyzer should have completed the full callback loop.
	az := h.users.analyzer
	if az.httpErr != nil {
		t.Fatalf("analyzer callback loop failed: %v", az.httpErr)
	}
	if az.calledModel != "test-model" {
		t.Fatalf("expected model passthrough, got %q", az.calledModel)
	}
	if len(az.schemaSeen) == 0 || len(az.schemaSeen["documents"]) == 0 {
		t.Fatalf("agent did not observe schema: %+v", az.schemaSeen)
	}
	if len(az.rowsSeen) != 1 {
		t.Fatalf("agent expected exactly this user's 1 row, got %d", len(az.rowsSeen))
	}

	// Sanity: structured JSON returned by the mock recognizer is valid JSON.
	var structured map[string]any
	if err := json.Unmarshal(doc.StructuredJSON, &structured); err != nil {
		t.Fatalf("structured_json invalid: %v", err)
	}
}
