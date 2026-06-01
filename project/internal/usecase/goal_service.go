package usecase

import (
	"context"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type GoalService struct {
	Goals  ports.GoalRepo
	Plans  ports.PlanRepo
	Groups ports.GroupRepo
}

type CreateGoalInput struct {
	Name        string
	Description *string
	SortOrder   int
}

func (s *GoalService) Create(ctx context.Context, ownerID, planID int64, in CreateGoalInput) (domain.Goal, error) {
	if _, err := requirePlan(ctx, s.Plans, s.Groups, planID, ownerID); err != nil {
		return domain.Goal{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return domain.Goal{}, ErrValidation
	}
	return s.Goals.Create(ctx, planID, name, in.Description, in.SortOrder)
}

func (s *GoalService) ListByPlan(ctx context.Context, ownerID, planID int64, page, size int) ([]domain.Goal, int, error) {
	if _, err := requirePlan(ctx, s.Plans, s.Groups, planID, ownerID); err != nil {
		return nil, 0, err
	}
	offset, limit := offsetLimit(page, size)
	return s.Goals.ListByPlan(ctx, planID, offset, limit)
}
