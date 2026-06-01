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

// ErrConflict is returned when an action is invalid for the resource's current
// state. It aliases the ports sentinel so conflicts raised in the repository
// layer (e.g. a failed conditional status transition) are matched here too.
var ErrConflict = ports.ErrConflict

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

// canAccess reports whether userID may act on a resource owned by ownerID.
// Access is granted when the caller is the owner, or when caller and owner share
// at least one corporate-account group ("all members share everything"). When
// groups is nil (e.g. in unit tests that don't wire group sharing), only direct
// ownership is allowed.
func canAccess(ctx context.Context, groups ports.GroupRepo, ownerID, userID int64) (bool, error) {
	if ownerID == userID {
		return true, nil
	}
	if groups == nil {
		return false, nil
	}
	peers, err := groups.CoMemberIDs(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, id := range peers {
		if id == ownerID {
			return true, nil
		}
	}
	return false, nil
}

// requirePlan loads a plan and verifies the caller may access it (owner or
// corporate-account co-member).
func requirePlan(ctx context.Context, plans ports.PlanRepo, groups ports.GroupRepo, planID, userID int64) (domain.Plan, error) {
	plan, err := plans.GetByID(ctx, planID)
	if err != nil {
		return domain.Plan{}, err
	}
	ok, err := canAccess(ctx, groups, plan.CreatedBy, userID)
	if err != nil {
		return domain.Plan{}, err
	}
	if !ok {
		return domain.Plan{}, ErrForbidden
	}
	return plan, nil
}

// requireGoal loads a goal under the given plan and verifies access.
func requireGoal(ctx context.Context, goals ports.GoalRepo, plans ports.PlanRepo, groups ports.GroupRepo, planID, goalID, userID int64) (domain.Goal, error) {
	goal, err := goals.GetByID(ctx, goalID)
	if err != nil {
		return domain.Goal{}, err
	}
	if goal.PlanID != planID {
		return domain.Goal{}, ErrNotFound
	}
	if _, err := requirePlan(ctx, plans, groups, planID, userID); err != nil {
		return domain.Goal{}, err
	}
	return goal, nil
}

// requireItemByID loads a plan item by id and verifies access via
// item → goal → plan → created_by (widened to corporate-account co-members).
func requireItemByID(ctx context.Context, items ports.PlanItemRepo, goals ports.GoalRepo, plans ports.PlanRepo, groups ports.GroupRepo, itemID, userID int64) (domain.PlanItem, error) {
	item, err := items.GetByID(ctx, itemID)
	if err != nil {
		return domain.PlanItem{}, err
	}
	goal, err := goals.GetByID(ctx, item.GoalID)
	if err != nil {
		return domain.PlanItem{}, err
	}
	if _, err := requirePlan(ctx, plans, groups, goal.PlanID, userID); err != nil {
		return domain.PlanItem{}, err
	}
	return item, nil
}
