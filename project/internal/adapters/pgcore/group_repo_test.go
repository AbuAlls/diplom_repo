//go:build integration

package pgcore

import (
	"context"
	"testing"
)

// seedUser inserts a minimal user row and returns its id.
func seedUser(t *testing.T, ctx context.Context, s *Store, email string) int64 {
	t.Helper()
	var id int64
	err := s.Pool.QueryRow(ctx,
		`insert into users (full_name, email, password_hash) values ($1, $2, 'x') returning id`,
		email, email,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seedUser %s: %v", email, err)
	}
	return id
}

func TestGroupRepo_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewGroupRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")

	g, err := r.Create(ctx, alice, "Acme Corp", "test account")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if g.ID == 0 || g.Name != "Acme Corp" || g.Role != "corporate" {
		t.Fatalf("unexpected group: %+v", g)
	}
	if g.CreatedBy == nil || *g.CreatedBy != alice {
		t.Fatalf("expected created_by=%d, got %v", alice, g.CreatedBy)
	}

	got, err := r.GetByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != g.Name {
		t.Fatalf("name mismatch: want %q got %q", g.Name, got.Name)
	}
}

func TestGroupRepo_CreatorIsAutoMember(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewGroupRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	g, _ := r.Create(ctx, alice, "Acme", "")

	ok, err := r.IsMember(ctx, g.ID, alice)
	if err != nil {
		t.Fatalf("IsMember: %v", err)
	}
	if !ok {
		t.Fatalf("creator must be auto-added as member")
	}
}

func TestGroupRepo_AddRemoveMember(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewGroupRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	bob := seedUser(t, ctx, store, "bob@corp.com")
	g, _ := r.Create(ctx, alice, "Acme", "")

	// Add Bob.
	if err := r.AddMember(ctx, g.ID, bob); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	ok, _ := r.IsMember(ctx, g.ID, bob)
	if !ok {
		t.Fatalf("bob must be a member after AddMember")
	}

	// Idempotent re-add must not error.
	if err := r.AddMember(ctx, g.ID, bob); err != nil {
		t.Fatalf("idempotent re-add: %v", err)
	}

	// Remove Bob.
	if err := r.RemoveMember(ctx, g.ID, bob); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	ok, _ = r.IsMember(ctx, g.ID, bob)
	if ok {
		t.Fatalf("bob must not be a member after RemoveMember")
	}
}

func TestGroupRepo_ListMembers(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewGroupRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	bob := seedUser(t, ctx, store, "bob@corp.com")
	g, _ := r.Create(ctx, alice, "Acme", "")
	_ = r.AddMember(ctx, g.ID, bob)

	members, err := r.ListMembers(ctx, g.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	emails := map[string]bool{}
	for _, m := range members {
		emails[m.Email] = true
	}
	if !emails["alice@corp.com"] || !emails["bob@corp.com"] {
		t.Fatalf("unexpected member emails: %v", emails)
	}
}

func TestGroupRepo_ListByMember(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewGroupRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	bob := seedUser(t, ctx, store, "bob@corp.com")
	g1, _ := r.Create(ctx, alice, "Acme", "")
	g2, _ := r.Create(ctx, alice, "BobCo", "")
	_ = r.AddMember(ctx, g2.ID, bob)

	// Alice is in both groups.
	groups, total, err := r.ListByMember(ctx, alice, 0, 10)
	if err != nil {
		t.Fatalf("ListByMember alice: %v", err)
	}
	if total != 2 || len(groups) != 2 {
		t.Fatalf("alice: expected 2 groups, got %d (total=%d)", len(groups), total)
	}

	// Bob is only in BobCo.
	groups, total, err = r.ListByMember(ctx, bob, 0, 10)
	if err != nil {
		t.Fatalf("ListByMember bob: %v", err)
	}
	if total != 1 || groups[0].ID != g2.ID {
		t.Fatalf("bob: expected 1 group (BobCo), got %d groups total=%d", len(groups), total)
	}

	// Alice is in g1 but not bob — so g1 should not appear.
	_ = g1
}

func TestGroupRepo_CoMemberIDs(t *testing.T) {
	ctx := context.Background()
	store := testDB(t)
	r := NewGroupRepo(store)

	alice := seedUser(t, ctx, store, "alice@corp.com")
	bob := seedUser(t, ctx, store, "bob@corp.com")
	carol := seedUser(t, ctx, store, "carol@corp.com")
	g, _ := r.Create(ctx, alice, "Acme", "")
	_ = r.AddMember(ctx, g.ID, bob)
	// Carol is in a different group.
	g2, _ := r.Create(ctx, carol, "Other", "")
	_ = g2

	peers, err := r.CoMemberIDs(ctx, alice)
	if err != nil {
		t.Fatalf("CoMemberIDs: %v", err)
	}
	peerSet := map[int64]bool{}
	for _, id := range peers {
		peerSet[id] = true
	}
	if !peerSet[alice] {
		t.Fatalf("alice must be in her own peer set")
	}
	if !peerSet[bob] {
		t.Fatalf("bob (co-member) must be in alice's peer set")
	}
	if peerSet[carol] {
		t.Fatalf("carol (different group) must NOT be in alice's peer set")
	}
}
