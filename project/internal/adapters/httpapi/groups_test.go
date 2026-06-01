package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestGroupCorporateAccountSharesFiles is the headline scenario: two users in a
// corporate-account group can see each other's plans and documents, while an
// outsider cannot.
func TestGroupCorporateAccountSharesFiles(t *testing.T) {
	srv, _ := newTestServerWithDeps()
	defer srv.Close()

	alice := register(t, srv.URL, "alice@corp.com")
	bob := register(t, srv.URL, "bob@corp.com")
	eve := register(t, srv.URL, "eve@outside.com") // not in the group

	// Alice creates a corporate account and adds Bob by email.
	resp := do(t, "POST", srv.URL+"/v0/groups", alice, `{"name":"Acme Corp"}`)
	var group groupResponse
	json.NewDecoder(resp.Body).Decode(&group)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || group.ID == 0 {
		t.Fatalf("create group: status=%d group=%+v", resp.StatusCode, group)
	}

	resp = do(t, "POST", srv.URL+"/v0/groups/1/members", alice, `{"email":"bob@corp.com"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add member: status=%d", resp.StatusCode)
	}

	// Alice builds a plan→goal→item and uploads a document.
	seedItem(t, srv.URL, alice)
	uploadDoc(t, srv.URL, alice, 1, "shared.pdf", "corporate data").Body.Close()

	// Bob (different account, same group) can read Alice's document.
	resp = do(t, "GET", srv.URL+"/v0/documents/1", bob, "")
	var doc documentResponse
	json.NewDecoder(resp.Body).Decode(&doc)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || doc.FileName != "shared.pdf" {
		t.Fatalf("bob should read shared doc: status=%d doc=%+v", resp.StatusCode, doc)
	}

	// Bob sees the document in his list view too (sharing widens the listing).
	resp = do(t, "GET", srv.URL+"/v0/documents", bob, "")
	var list listResponse
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Meta.Total != 1 {
		t.Fatalf("bob should see 1 shared document, got total=%d", list.Meta.Total)
	}

	// Bob sees Alice's plan in his plan list.
	resp = do(t, "GET", srv.URL+"/v0/plans", bob, "")
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Meta.Total != 1 {
		t.Fatalf("bob should see 1 shared plan, got total=%d", list.Meta.Total)
	}

	// Bob can act on the shared document — confirm it.
	resp = do(t, "POST", srv.URL+"/v0/documents/1/confirm", bob, "")
	json.NewDecoder(resp.Body).Decode(&doc)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || doc.Status != "confirmed" {
		t.Fatalf("bob should confirm shared doc: status=%d doc=%+v", resp.StatusCode, doc)
	}

	// Eve (outside the group) is forbidden from the document and sees no plans.
	resp = do(t, "GET", srv.URL+"/v0/documents/1", eve, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("eve must be forbidden from shared doc, got %d", resp.StatusCode)
	}
	resp = do(t, "GET", srv.URL+"/v0/plans", eve, "")
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Meta.Total != 0 {
		t.Fatalf("eve must see no shared plans, got total=%d", list.Meta.Total)
	}
}

func TestGroupOnlyCreatorAddsMembers(t *testing.T) {
	srv, _ := newTestServerWithDeps()
	defer srv.Close()

	alice := register(t, srv.URL, "alice@corp.com")
	bob := register(t, srv.URL, "bob@corp.com")
	register(t, srv.URL, "carol@corp.com")

	resp := do(t, "POST", srv.URL+"/v0/groups", alice, `{"name":"Acme"}`)
	resp.Body.Close()

	// Bob is not a member, let alone the creator — he cannot add Carol.
	resp = do(t, "POST", srv.URL+"/v0/groups/1/members", bob, `{"email":"carol@corp.com"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-creator must not add members, got %d", resp.StatusCode)
	}

	// Adding an unknown email yields 404.
	resp = do(t, "POST", srv.URL+"/v0/groups/1/members", alice, `{"email":"nobody@nowhere.com"}`)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("adding unknown email should 404, got %d", resp.StatusCode)
	}
}

func TestGroupListAndGet(t *testing.T) {
	srv, _ := newTestServerWithDeps()
	defer srv.Close()

	alice := register(t, srv.URL, "alice@corp.com")
	register(t, srv.URL, "bob@corp.com")

	resp := do(t, "POST", srv.URL+"/v0/groups", alice, `{"name":"Acme","description":"books"}`)
	resp.Body.Close()
	resp = do(t, "POST", srv.URL+"/v0/groups/1/members", alice, `{"email":"bob@corp.com"}`)
	resp.Body.Close()

	// Creator lists their groups.
	resp = do(t, "GET", srv.URL+"/v0/groups", alice, "")
	var list listResponse
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Meta.Total != 1 {
		t.Fatalf("expected 1 group, got %d", list.Meta.Total)
	}

	// Group detail includes both members.
	resp = do(t, "GET", srv.URL+"/v0/groups/1", alice, "")
	var detail groupDetailResponse
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(detail.Members) != 2 {
		t.Fatalf("expected 2 members, got status=%d members=%d", resp.StatusCode, len(detail.Members))
	}
}

func TestGroupNonMemberCannotGet(t *testing.T) {
	srv, _ := newTestServerWithDeps()
	defer srv.Close()

	alice := register(t, srv.URL, "alice@corp.com")
	eve := register(t, srv.URL, "eve@outside.com")

	resp := do(t, "POST", srv.URL+"/v0/groups", alice, `{"name":"Acme"}`)
	resp.Body.Close()

	resp = do(t, "GET", srv.URL+"/v0/groups/1", eve, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("non-member must not read group, got %d", resp.StatusCode)
	}
}
