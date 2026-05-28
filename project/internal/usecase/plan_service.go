package usecase

import (
	"context"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

const defaultPlanStatus = "active"

type PlanService struct {
	Plans ports.PlanRepo
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

func (s *PlanService) List(ctx context.Context, ownerID int64, page, size int) ([]domain.Plan, int, error) {
	offset, limit := offsetLimit(page, size)
	return s.Plans.ListByOwner(ctx, ownerID, offset, limit)
}
