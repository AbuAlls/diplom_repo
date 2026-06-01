package usecase

import (
	"context"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

const (
	defaultItemType   = "generic"
	defaultItemStatus = "active"
)

type PlanItemService struct {
	Items  ports.PlanItemRepo
	Goals  ports.GoalRepo
	Plans  ports.PlanRepo
	Groups ports.GroupRepo
}

func (s *PlanItemService) Create(ctx context.Context, ownerID, planID, goalID int64, in ports.PlanItemInput) (domain.PlanItem, error) {
	if _, err := requireGoal(ctx, s.Goals, s.Plans, s.Groups, planID, goalID, ownerID); err != nil {
		return domain.PlanItem{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return domain.PlanItem{}, ErrValidation
	}
	if strings.TrimSpace(in.ItemType) == "" {
		in.ItemType = defaultItemType
	}
	if strings.TrimSpace(in.Status) == "" {
		in.Status = defaultItemStatus
	}
	return s.Items.Create(ctx, goalID, in)
}

func (s *PlanItemService) ListByGoal(ctx context.Context, ownerID, planID, goalID int64, page, size int) ([]domain.PlanItem, int, error) {
	if _, err := requireGoal(ctx, s.Goals, s.Plans, s.Groups, planID, goalID, ownerID); err != nil {
		return nil, 0, err
	}
	offset, limit := offsetLimit(page, size)
	return s.Items.ListByGoal(ctx, goalID, offset, limit)
}

func (s *PlanItemService) Update(ctx context.Context, ownerID, planID, goalID, itemID int64, patch ports.PlanItemPatch) (domain.PlanItem, error) {
	if _, err := requireGoal(ctx, s.Goals, s.Plans, s.Groups, planID, goalID, ownerID); err != nil {
		return domain.PlanItem{}, err
	}
	item, err := s.Items.GetByID(ctx, itemID)
	if err != nil {
		return domain.PlanItem{}, err
	}
	if item.GoalID != goalID {
		return domain.PlanItem{}, ErrNotFound
	}
	return s.Items.Update(ctx, itemID, patch)
}
