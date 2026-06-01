package usecase

import (
	"context"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

const defaultPlanStatus = "active"

type PlanService struct {
	Plans  ports.PlanRepo
	Groups ports.GroupRepo
}

type CreatePlanInput struct {
	Name        string
	Description *string
	Status      string
}

func (s *PlanService) Create(ctx context.Context, ownerID int64, in CreatePlanInput) (domain.Plan, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Plan{}, ErrValidation
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = defaultPlanStatus
	}
	return s.Plans.Create(ctx, ownerID, name, in.Description, status)
}

// List returns the plans visible to userID: their own plus those owned by any
// corporate-account co-member. Without a group wired (Groups == nil) it falls
// back to the user's own plans.
func (s *PlanService) List(ctx context.Context, userID int64, page, size int) ([]domain.Plan, int, error) {
	offset, limit := offsetLimit(page, size)
	if s.Groups == nil {
		return s.Plans.ListByOwner(ctx, userID, offset, limit)
	}
	owners, err := s.Groups.CoMemberIDs(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return s.Plans.ListByOwners(ctx, owners, offset, limit)
}
