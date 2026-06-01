package usecase

import (
	"context"
	"sync"
	"testing"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

// stubGroupRepo is a minimal in-memory GroupRepo for use-case tests.
type stubGroupRepo struct {
	mu      sync.Mutex
	byID    map[int64]domain.Group
	members map[int64]map[int64]bool
	nextID  int64
}

func newStubGroupRepo() *stubGroupRepo {
	return &stubGroupRepo{byID: map[int64]domain.Group{}, members: map[int64]map[int64]bool{}}
}

func (s *stubGroupRepo) Create(_ context.Context, createdBy int64, name, description string) (domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	cb := createdBy
	g := domain.Group{ID: s.nextID, Name: name, Description: description, Role: "corporate", CreatedBy: &cb}
	s.byID[g.ID] = g
	s.members[g.ID] = map[int64]bool{createdBy: true}
	return g, nil
}
func (s *stubGroupRepo) GetByID(_ context.Context, id int64) (domain.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.byID[id]
	if !ok {
		return domain.Group{}, ports.ErrNotFound
	}
	return g, nil
}
func (s *stubGroupRepo) ListByMember(_ context.Context, userID int64, _, _ int) ([]domain.Group, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Group
	for gid, set := range s.members {
		if set[userID] {
			out = append(out, s.byID[gid])
		}
	}
	return out, len(out), nil
}
func (s *stubGroupRepo) AddMember(_ context.Context, groupID, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.members[groupID] == nil {
		s.members[groupID] = map[int64]bool{}
	}
	s.members[groupID][userID] = true
	return nil
}
func (s *stubGroupRepo) RemoveMember(_ context.Context, groupID, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.members[groupID], userID)
	return nil
}
func (s *stubGroupRepo) ListMembers(_ context.Context, groupID int64) ([]domain.GroupMember, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.GroupMember
	for uid := range s.members[groupID] {
		out = append(out, domain.GroupMember{UserID: uid, GroupID: groupID})
	}
	return out, nil
}
func (s *stubGroupRepo) IsMember(_ context.Context, groupID, userID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.members[groupID][userID], nil
}
func (s *stubGroupRepo) CoMemberIDs(_ context.Context, userID int64) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []int64{userID}
	seen := map[int64]bool{userID: true}
	for gid, set := range s.members {
		if !set[userID] {
			continue
		}
		for uid := range s.members[gid] {
			if !seen[uid] {
				seen[uid] = true
				ids = append(ids, uid)
			}
		}
	}
	return ids, nil
}

// stubUserRepo resolves emails to users for AddMemberByEmail.
type stubUserRepo struct {
	byEmail map[string]ports.UserDTO
}

func (s stubUserRepo) Create(context.Context, string, string, string) (int64, error) { return 0, nil }
func (s stubUserRepo) GetByEmail(_ context.Context, email string) (ports.UserDTO, error) {
	u, ok := s.byEmail[email]
	if !ok {
		return ports.UserDTO{}, ports.ErrNotFound
	}
	return u, nil
}
func (s stubUserRepo) GetByID(context.Context, int64) (ports.UserDTO, error) {
	return ports.UserDTO{}, ports.ErrNotFound
}

func newGroupSvc() (*GroupService, *stubGroupRepo) {
	groups := newStubGroupRepo()
	users := stubUserRepo{byEmail: map[string]ports.UserDTO{
		"bob@corp.com":   {ID: 2, Email: "bob@corp.com", FullName: "Bob"},
		"carol@corp.com": {ID: 3, Email: "carol@corp.com", FullName: "Carol"},
	}}
	return &GroupService{Groups: groups, Users: users}, groups
}

func TestGroupCreateMakesCreatorMember(t *testing.T) {
	svc, groups := newGroupSvc()
	g, err := svc.Create(context.Background(), 1, CreateGroupInput{Name: "Acme"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ok, _ := groups.IsMember(context.Background(), g.ID, 1)
	if !ok {
		t.Fatalf("creator should be a member")
	}
}

func TestGroupCreateRejectsEmptyName(t *testing.T) {
	svc, _ := newGroupSvc()
	if _, err := svc.Create(context.Background(), 1, CreateGroupInput{Name: "  "}); err != ErrValidation {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestGroupAddMemberByEmail(t *testing.T) {
	svc, groups := newGroupSvc()
	g, _ := svc.Create(context.Background(), 1, CreateGroupInput{Name: "Acme"})

	m, err := svc.AddMemberByEmail(context.Background(), 1, g.ID, "bob@corp.com")
	if err != nil {
		t.Fatalf("add member: %v", err)
	}
	if m.UserID != 2 {
		t.Fatalf("expected bob (id 2), got %d", m.UserID)
	}
	if ok, _ := groups.IsMember(context.Background(), g.ID, 2); !ok {
		t.Fatalf("bob should now be a member")
	}
}

func TestGroupOnlyCreatorAddsMembers(t *testing.T) {
	svc, _ := newGroupSvc()
	g, _ := svc.Create(context.Background(), 1, CreateGroupInput{Name: "Acme"})

	// User 2 is not the creator → forbidden.
	if _, err := svc.AddMemberByEmail(context.Background(), 2, g.ID, "carol@corp.com"); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden for non-creator, got %v", err)
	}
}

func TestGroupAddUnknownEmail(t *testing.T) {
	svc, _ := newGroupSvc()
	g, _ := svc.Create(context.Background(), 1, CreateGroupInput{Name: "Acme"})
	if _, err := svc.AddMemberByEmail(context.Background(), 1, g.ID, "ghost@corp.com"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for unknown email, got %v", err)
	}
}

func TestGroupCreatorCannotRemoveSelf(t *testing.T) {
	svc, _ := newGroupSvc()
	g, _ := svc.Create(context.Background(), 1, CreateGroupInput{Name: "Acme"})
	if err := svc.RemoveMember(context.Background(), 1, g.ID, 1); err != ErrConflict {
		t.Fatalf("expected ErrConflict removing self, got %v", err)
	}
}

func TestGroupRemoveMember(t *testing.T) {
	svc, groups := newGroupSvc()
	g, _ := svc.Create(context.Background(), 1, CreateGroupInput{Name: "Acme"})
	_, _ = svc.AddMemberByEmail(context.Background(), 1, g.ID, "bob@corp.com")

	if err := svc.RemoveMember(context.Background(), 1, g.ID, 2); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if ok, _ := groups.IsMember(context.Background(), g.ID, 2); ok {
		t.Fatalf("bob should have been removed")
	}
}

// TestAccessWidensToCoMembers checks the access helper directly: a co-member is
// granted, an outsider is denied, and nil groups means owner-only.
func TestAccessWidensToCoMembers(t *testing.T) {
	groups := newStubGroupRepo()
	g, _ := groups.Create(context.Background(), 1, "Acme", "")
	_ = groups.AddMember(context.Background(), g.ID, 2)

	// 2 shares a group with 1 → can access 1's resources.
	ok, err := canAccess(context.Background(), groups, 1, 2)
	if err != nil || !ok {
		t.Fatalf("co-member should have access, ok=%v err=%v", ok, err)
	}
	// 3 is not in any group → denied.
	ok, _ = canAccess(context.Background(), groups, 1, 3)
	if ok {
		t.Fatalf("outsider must be denied")
	}
	// Owner always allowed.
	ok, _ = canAccess(context.Background(), groups, 1, 1)
	if !ok {
		t.Fatalf("owner must be allowed")
	}
	// nil groups → owner-only.
	ok, _ = canAccess(context.Background(), nil, 1, 2)
	if ok {
		t.Fatalf("with nil groups, only the owner is allowed")
	}
}
