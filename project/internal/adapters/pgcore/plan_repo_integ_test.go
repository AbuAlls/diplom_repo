//go:build integration

package pgcore

import (
	"context"
	"testing"
)

func TestPlanRepo_ListByOwners(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewPlanRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	bob := seedUser(t, ctx, store, "bob@corp.com")
	eve := seedUser(t, ctx, store, "eve@outside.com")

	for _, name := range []string{"Plan A", "Plan B"} {
		if _, err := r.Create(ctx, alice, name, nil, "active"); err != nil {
			t.Fatalf("create alice plan: %v", err)
		}
	}
	if _, err := r.Create(ctx, bob, "Bob Plan", nil, "active"); err != nil {
		t.Fatalf("create bob plan: %v", err)
	}
	if _, err := r.Create(ctx, eve, "Eve Plan", nil, "active"); err != nil {
		t.Fatalf("create eve plan: %v", err)
	}

	// Alice + Bob together see 3 plans, not Eve's.
	plans, total, err := r.ListByOwners(ctx, []int64{alice, bob}, 0, 10)
	if err != nil {
		t.Fatalf("ListByOwners: %v", err)
	}
	if total != 3 || len(plans) != 3 {
		t.Fatalf("expected 3 plans (alice+bob), got %d (total=%d)", len(plans), total)
	}
	for _, p := range plans {
		if p.CreatedBy != alice && p.CreatedBy != bob {
			t.Fatalf("eve's plan leaked into alice+bob listing: %+v", p)
		}
	}

	// Pagination: page 2 of size 2 → 1 remaining plan.
	plans, total, err = r.ListByOwners(ctx, []int64{alice, bob}, 2, 2)
	if err != nil {
		t.Fatalf("ListByOwners page2: %v", err)
	}
	if total != 3 || len(plans) != 1 {
		t.Fatalf("page 2: expected total=3 len=1, got total=%d len=%d", total, len(plans))
	}

	// Empty ownerIDs → no results.
	plans, total, err = r.ListByOwners(ctx, nil, 0, 10)
	if err != nil {
		t.Fatalf("ListByOwners(nil): %v", err)
	}
	if total != 0 || len(plans) != 0 {
		t.Fatalf("expected 0 for nil owners, got %d", len(plans))
	}
}
