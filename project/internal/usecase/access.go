package usecase

import (
	"context"
	"errors"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

// ErrForbidden is returned when a resource exists but the caller does not own it.
var ErrForbidden = errors.New("forbidden")

// ErrValidation is returned when input fails business validation.
var ErrValidation = errors.New("validation error")

// ErrConflict is returned when an action is invalid for the resource's current state.
var ErrConflict = errors.New("conflict")

// offsetLimit normalizes page/size (1-based page, size clamped 1..100) into an
// SQL offset/limit pair.
func offsetLimit(page, size int) (offset, limit int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return (page - 1) * size, size
}

// ErrNotFound re-exports the repository sentinel for handler mapping.
var ErrNotFound = ports.ErrNotFound

// requirePlan loads a plan and verifies the caller owns it.
func requirePlan(ctx context.Context, plans ports.PlanRepo, planID, userID int64) (domain.Plan, error) {
	plan, err := plans.GetByID(ctx, planID)
	if err != nil {
		return domain.Plan{}, err
	}
	if plan.CreatedBy != userID {
		return domain.Plan{}, ErrForbidden
	}
	return plan, nil
}

// requireGoal loads a goal under the given plan and verifies ownership.
func requireGoal(ctx context.Context, goals ports.GoalRepo, plans ports.PlanRepo, planID, goalID, userID int64) (domain.Goal, error) {
	goal, err := goals.GetByID(ctx, goalID)
	if err != nil {
		return domain.Goal{}, err
	}
	if goal.PlanID != planID {
		return domain.Goal{}, ErrNotFound
	}
	if _, err := requirePlan(ctx, plans, planID, userID); err != nil {
		return domain.Goal{}, err
	}
	return goal, nil
}

// requireItemByID loads a plan item by id and verifies the caller owns it via
// item → goal → plan → created_by.
func requireItemByID(ctx context.Context, items ports.PlanItemRepo, goals ports.GoalRepo, plans ports.PlanRepo, itemID, userID int64) (domain.PlanItem, error) {
	item, err := items.GetByID(ctx, itemID)
	if err != nil {
		return domain.PlanItem{}, err
	}
	goal, err := goals.GetByID(ctx, item.GoalID)
	if err != nil {
		return domain.PlanItem{}, err
	}
	if _, err := requirePlan(ctx, plans, goal.PlanID, userID); err != nil {
		return domain.PlanItem{}, err
	}
	return item, nil
}
