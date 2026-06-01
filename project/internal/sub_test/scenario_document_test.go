package sub_test

import (
	"net/http"
	"testing"
)

// Scenario 2 — Document review state machine. Exercises every document
// transition endpoint and asserts the legal/illegal moves:
//
//	upload → pending_review
//	confirm (from pending_review) → confirmed          [legal]
//	confirm again → 409                                 [illegal: terminal]
//	reanalyze a confirmed doc → 409                     [illegal]
//
// then, on a second document:
//
//	reject (from pending_review) → rejected
//	confirm a rejected doc → 409                        [illegal]
//	reanalyze a rejected doc → pending_review           [legal, re-opens]
//	patch + confirm → confirmed                         [legal]
//
// Also walks the read side: list (with plan_item filter), get, storage HEAD and
// download, so a document's bytes and metadata are verified end to end.
func TestScenario_DocumentStateMachine(t *testing.T) {
	h := newHarness(t)
	token := h.register("reviewer@example.com")
	_, _, itemID := h.seedItem(token, "Compliance", 10, 0)

	// ---- document #1: the confirm path ----
	st, d1 := h.uploadDoc(token, itemID, "first.pdf", "first-bytes")
	if st != http.StatusOK || d1.Status != "pending_review" {
		t.Fatalf("upload d1: status=%d docStatus=%q", st, d1.Status)
	}

	// storage HEAD reflects the saved object.
	var storage storageResp
	if st := h.reqJSON("GET", "/v0/documents/"+itoa(d1.ID)+"/storage", token, "", &storage); st != http.StatusOK {
		t.Fatalf("storage: status %d", st)
	}
	if !storage.ObjectExists || storage.ObjectStatusCode != http.StatusOK {
		t.Fatalf("expected object to exist: %+v", storage)
	}
	if storage.ObjectSize == nil || *storage.ObjectSize != int64(len("first-bytes")) {
		t.Fatalf("unexpected object size: %+v", storage)
	}

	// download returns the exact bytes that were uploaded.
	dlSt, dlBody := h.req("GET", "/v0/documents/"+itoa(d1.ID)+"/download", token, "")
	if dlSt != http.StatusOK || string(dlBody) != "first-bytes" {
		t.Fatalf("download mismatch: status=%d body=%q", dlSt, dlBody)
	}

	if st, _ := h.req("POST", "/v0/documents/"+itoa(d1.ID)+"/confirm", token, ""); st != http.StatusOK {
		t.Fatalf("confirm d1: status %d", st)
	}
	if st, body := h.req("POST", "/v0/documents/"+itoa(d1.ID)+"/confirm", token, ""); st != http.StatusConflict {
		t.Fatalf("double confirm: expected 409, got %d", st)
	} else if code := errorCode(t, body); code != "CONFLICT" {
		t.Fatalf("double confirm: expected CONFLICT code, got %q", code)
	}
	if st, _ := h.req("POST", "/v0/documents/"+itoa(d1.ID)+"/reanalyze", token, ""); st != http.StatusConflict {
		t.Fatalf("reanalyze confirmed: expected 409, got %d", st)
	}

	// ---- document #2: the reject → reanalyze → confirm path ----
	st, d2 := h.uploadDoc(token, itemID, "second.pdf", "second-bytes")
	if st != http.StatusOK {
		t.Fatalf("upload d2: status %d", st)
	}

	var rejected docResp
	if st := h.reqJSON("POST", "/v0/documents/"+itoa(d2.ID)+"/reject", token, "", &rejected); st != http.StatusOK {
		t.Fatalf("reject d2: status %d", st)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("expected rejected, got %q", rejected.Status)
	}
	if st, _ := h.req("POST", "/v0/documents/"+itoa(d2.ID)+"/confirm", token, ""); st != http.StatusConflict {
		t.Fatalf("confirm rejected: expected 409, got %d", st)
	}

	var reanalyzed docResp
	if st := h.reqJSON("POST", "/v0/documents/"+itoa(d2.ID)+"/reanalyze", token, "", &reanalyzed); st != http.StatusOK {
		t.Fatalf("reanalyze d2: status %d", st)
	}
	if reanalyzed.Status != "pending_review" {
		t.Fatalf("expected pending_review after reanalyze, got %q", reanalyzed.Status)
	}
	if reanalyzed.RecognizedText == "" {
		t.Fatalf("expected recognized text after reanalyze")
	}

	// An empty PATCH body is rejected (at least one field required).
	if st, body := h.req("PATCH", "/v0/documents/"+itoa(d2.ID), token, `{}`); st != http.StatusBadRequest {
		t.Fatalf("empty patch: expected 400, got %d", st)
	} else if code := errorCode(t, body); code != "VALIDATION_ERROR" {
		t.Fatalf("empty patch: expected VALIDATION_ERROR, got %q", code)
	}

	if st := h.reqJSON("PATCH", "/v0/documents/"+itoa(d2.ID), token, `{"external_number":"INV-77"}`, &reanalyzed); st != http.StatusOK {
		t.Fatalf("patch d2: status %d", st)
	}
	if reanalyzed.ExternalNumber == nil || *reanalyzed.ExternalNumber != "INV-77" {
		t.Fatalf("external_number not persisted: %v", reanalyzed.ExternalNumber)
	}
	if st, _ := h.req("POST", "/v0/documents/"+itoa(d2.ID)+"/confirm", token, ""); st != http.StatusOK {
		t.Fatalf("final confirm d2: status %d", st)
	}

	// ---- read side: both documents belong to the item ----
	var list listResp
	if st := h.reqJSON("GET", "/v0/documents?plan_item_id="+itoa(itemID), token, "", &list); st != http.StatusOK {
		t.Fatalf("list documents: status %d", st)
	}
	if list.Meta.Total != 2 {
		t.Fatalf("expected 2 documents for the item, got %d", list.Meta.Total)
	}

	// analytics now counts both documents as sources.
	var an analyticsResp
	h.reqJSON("GET", "/v0/items/"+itoa(itemID)+"/analytics", token, "", &an)
	if an.SourceDocumentsCount != 2 {
		t.Fatalf("expected 2 source documents, got %d", an.SourceDocumentsCount)
	}
}
