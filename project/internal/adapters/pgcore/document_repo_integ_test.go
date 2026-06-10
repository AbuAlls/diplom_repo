//go:build integration

package pgcore

import (
	"context"
	"testing"

	"diplom.com/m/internal/ports"
)

// seedFolder creates the minimal plan → goal → item → folder chain needed for a
// document row, and returns (planItemID, folderID).
func seedFolder(t *testing.T, ctx context.Context, s *Store, ownerID int64) (planItemID, folderID int64) {
	t.Helper()
	var planID int64
	if err := s.Pool.QueryRow(ctx,
		`insert into plans (name, description, created_by, status) values ('p','',  $1,'active') returning id`,
		ownerID,
	).Scan(&planID); err != nil {
		t.Fatalf("seedFolder plan: %v", err)
	}
	var goalID int64
	if err := s.Pool.QueryRow(ctx,
		`insert into plan_goals (plan_id, name) values ($1, 'g') returning id`,
		planID,
	).Scan(&goalID); err != nil {
		t.Fatalf("seedFolder goal: %v", err)
	}
	if err := s.Pool.QueryRow(ctx,
		`insert into plan_items (goal_id, name, item_type, status) values ($1, 'it','generic','active') returning id`,
		goalID,
	).Scan(&planItemID); err != nil {
		t.Fatalf("seedFolder item: %v", err)
	}
	if err := s.Pool.QueryRow(ctx,
		`insert into folders (name, created_by, is_system, plan_item_id) values ('f', $1, true, $2) returning id`,
		ownerID, planItemID,
	).Scan(&folderID); err != nil {
		t.Fatalf("seedFolder folder: %v", err)
	}
	return
}

// seedDocument creates one document row and returns its ID.
func seedDocument(t *testing.T, ctx context.Context, r *DocumentRepo, ownerID, folderID int64, status string) int64 {
	t.Helper()
	doc, err := r.Create(ctx, ports.DocumentCreate{
		FolderID:   folderID,
		UploadedBy: ownerID,
		Title:      "test.pdf",
		Status:     status,
		FileName:   "test.pdf",
		FilePath:   "test/test.pdf",
		MimeType:   "application/pdf",
	})
	if err != nil {
		t.Fatalf("seedDocument: %v", err)
	}
	return doc.ID
}

// TestDocumentRepo_ListByOwners verifies the multi-owner query returns only rows
// belonging to the requested owner IDs and respects the optional planItemID filter.
func TestDocumentRepo_ListByOwners(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewDocumentRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	bob := seedUser(t, ctx, store, "bob@corp.com")
	eve := seedUser(t, ctx, store, "eve@outside.com")

	_, afolder := seedFolder(t, ctx, store, alice)
	_, bfolder := seedFolder(t, ctx, store, bob)
	_, efolder := seedFolder(t, ctx, store, eve)

	seedDocument(t, ctx, r, alice, afolder, "uploaded")
	seedDocument(t, ctx, r, alice, afolder, "confirmed")
	seedDocument(t, ctx, r, bob, bfolder, "pending_review")
	seedDocument(t, ctx, r, eve, efolder, "uploaded") // must not appear

	// Alice + Bob together (corporate account): should see 3 docs.
	docs, total, err := r.ListByOwners(ctx, []int64{alice, bob}, nil, 0, 10)
	if err != nil {
		t.Fatalf("ListByOwners: %v", err)
	}
	if total != 3 || len(docs) != 3 {
		t.Fatalf("expected 3 docs for alice+bob, got %d (total=%d)", len(docs), total)
	}
	for _, d := range docs {
		if d.UploadedBy != alice && d.UploadedBy != bob {
			t.Fatalf("eve's doc leaked into alice+bob listing: %+v", d)
		}
	}

	// Empty ownerIDs → no results (not a DB error).
	docs, total, err = r.ListByOwners(ctx, nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("ListByOwners(nil): %v", err)
	}
	if total != 0 || len(docs) != 0 {
		t.Fatalf("expected 0 docs for nil owners, got %d", len(docs))
	}
}

// TestDocumentRepo_UpdateStatusSerializable_Conditional checks that the conditional
// guard (expectFrom) is honoured: the update succeeds when current status matches
// and returns ErrConflict when it doesn't.
func TestDocumentRepo_UpdateStatusSerializable_Conditional(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewDocumentRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	_, folder := seedFolder(t, ctx, store, alice)
	docID := seedDocument(t, ctx, r, alice, folder, "uploaded")

	// Transition uploaded → processing with correct expectFrom.
	doc, err := r.UpdateStatusSerializable(ctx, docID, "processing", "uploaded", "failed")
	if err != nil {
		t.Fatalf("UpdateStatusSerializable: %v", err)
	}
	if doc.Status != "processing" {
		t.Fatalf("expected processing, got %q", doc.Status)
	}

	// Now try to re-confirm from processing using a wrong expectFrom (only pending_review).
	_, err = r.UpdateStatusSerializable(ctx, docID, "confirmed", "pending_review")
	if err != ports.ErrConflict {
		t.Fatalf("expected ErrConflict for wrong expectFrom, got %v", err)
	}

	// No expectFrom → unconditional update always applies.
	doc, err = r.UpdateStatusSerializable(ctx, docID, "confirmed")
	if err != nil {
		t.Fatalf("unconditional UpdateStatusSerializable: %v", err)
	}
	if doc.Status != "confirmed" {
		t.Fatalf("expected confirmed, got %q", doc.Status)
	}
}

// TestDocumentRepo_UpdateStatusSerializable_ReturnedDoc checks that the full
// document row (including plan_item_id resolved via folder join) is returned,
// not just the status update.
func TestDocumentRepo_UpdateStatusSerializable_ReturnedDoc(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewDocumentRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	planItemID, folder := seedFolder(t, ctx, store, alice)
	docID := seedDocument(t, ctx, r, alice, folder, "uploaded")

	doc, err := r.UpdateStatusSerializable(ctx, docID, "processing", "uploaded")
	if err != nil {
		t.Fatalf("UpdateStatusSerializable: %v", err)
	}
	if doc.ID != docID {
		t.Fatalf("returned wrong doc ID: %d", doc.ID)
	}
	if doc.PlanItemID != planItemID {
		t.Fatalf("plan_item_id not populated from folder join: got %d want %d", doc.PlanItemID, planItemID)
	}
	if doc.UploadedBy != alice {
		t.Fatalf("uploaded_by not returned: got %d", doc.UploadedBy)
	}
}
