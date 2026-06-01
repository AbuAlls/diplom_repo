package usecase

import (
	"context"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

// GroupService manages corporate-account groups: shared workspaces whose members
// can all see one another's plans and documents. The creator is the first member
// and the only one allowed to add or remove members (creator-as-admin model).
type GroupService struct {
	Groups ports.GroupRepo
	Users  ports.UserRepo
}

type CreateGroupInput struct {
	Name        string
	Description string
}

func (s *GroupService) Create(ctx context.Context, ownerID int64, in CreateGroupInput) (domain.Group, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Group{}, ErrValidation
	}
	return s.Groups.Create(ctx, ownerID, name, strings.TrimSpace(in.Description))
}

func (s *GroupService) List(ctx context.Context, userID int64, page, size int) ([]domain.Group, int, error) {
	offset, limit := offsetLimit(page, size)
	return s.Groups.ListByMember(ctx, userID, offset, limit)
}

// Get returns a group the caller belongs to, with its members. Non-members get
// ErrForbidden (ErrNotFound if the group doesn't exist).
func (s *GroupService) Get(ctx context.Context, userID, groupID int64) (domain.Group, []domain.GroupMember, error) {
	g, err := s.requireMember(ctx, groupID, userID)
	if err != nil {
		return domain.Group{}, nil, err
	}
	members, err := s.Groups.ListMembers(ctx, groupID)
	if err != nil {
		return domain.Group{}, nil, err
	}
	return g, members, nil
}

// AddMemberByEmail adds an existing user (looked up by email) to the group. Only
// the group creator may add members.
func (s *GroupService) AddMemberByEmail(ctx context.Context, callerID, groupID int64, email string) (domain.GroupMember, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return domain.GroupMember{}, ErrValidation
	}
	g, err := s.requireCreator(ctx, groupID, callerID)
	if err != nil {
		return domain.GroupMember{}, err
	}
	user, err := s.Users.GetByEmail(ctx, email)
	if err != nil {
		// Don't leak whether the email exists; treat as a validation failure.
		return domain.GroupMember{}, ErrNotFound
	}
	if err := s.Groups.AddMember(ctx, g.ID, user.ID); err != nil {
		return domain.GroupMember{}, err
	}
	return domain.GroupMember{UserID: user.ID, GroupID: g.ID, Email: user.Email, FullName: user.FullName}, nil
}

// RemoveMember removes a member from the group. Only the creator may remove
// members, and the creator cannot remove themselves (keeps every group with an
// admin).
func (s *GroupService) RemoveMember(ctx context.Context, callerID, groupID, memberID int64) error {
	g, err := s.requireCreator(ctx, groupID, callerID)
	if err != nil {
		return err
	}
	if g.CreatedBy != nil && *g.CreatedBy == memberID {
		return ErrConflict
	}
	return s.Groups.RemoveMember(ctx, groupID, memberID)
}

func (s *GroupService) requireMember(ctx context.Context, groupID, userID int64) (domain.Group, error) {
	g, err := s.Groups.GetByID(ctx, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	ok, err := s.Groups.IsMember(ctx, groupID, userID)
	if err != nil {
		return domain.Group{}, err
	}
	if !ok {
		return domain.Group{}, ErrForbidden
	}
	return g, nil
}

func (s *GroupService) requireCreator(ctx context.Context, groupID, userID int64) (domain.Group, error) {
	g, err := s.Groups.GetByID(ctx, groupID)
	if err != nil {
		return domain.Group{}, err
	}
	if g.CreatedBy == nil || *g.CreatedBy != userID {
		return domain.Group{}, ErrForbidden
	}
	return g, nil
}
