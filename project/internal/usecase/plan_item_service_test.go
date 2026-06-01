package usecase

import (
	"context"
	"testing"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type stubPlanRepo struct {
	plan domain.Plan
	err  error
}

func (s stubPlanRepo) Create(context.Context, int64, string, *string, string) (domain.Plan, error) {
	return domain.Plan{}, nil
}
func (s stubPlanRepo) GetByID(context.Context, int64) (domain.Plan, error) { return s.plan, s.err }
func (s stubPlanRepo) ListByOwner(context.Context, int64, int, int) ([]domain.Plan, int, error) {
	return nil, 0, nil
}
func (s stubPlanRepo) ListByOwners(context.Context, []int64, int, int) ([]domain.Plan, int, error) {
	return nil, 0, nil
}

type stubGoalRepo struct {
	goal domain.Goal
	err  error
}

func (s stubGoalRepo) Create(context.Context, int64, string, *string, int) (domain.Goal, error) {
	return domain.Goal{}, nil
}
func (s stubGoalRepo) GetByID(context.Context, int64) (domain.Goal, error) { return s.goal, s.err }
func (s stubGoalRepo) ListByPlan(context.Context, int64, int, int) ([]domain.Goal, int, error) {
	return nil, 0, nil
}

type stubItemRepo struct {
	created  ports.PlanItemInput
	getItem  domain.PlanItem
	getErr   error
	updateID int64
}

func (s *stubItemRepo) Create(_ context.Context, _ int64, in ports.PlanItemInput) (domain.PlanItem, error) {
	s.created = in
	return domain.PlanItem{ID: 1, Name: in.Name, ItemType: &in.ItemType, Status: &in.Status}, nil
}
func (s *stubItemRepo) GetByID(context.Context, int64) (domain.PlanItem, error) {
	return s.getItem, s.getErr
}
func (s *stubItemRepo) ListByGoal(context.Context, int64, int, int) ([]domain.PlanItem, int, error) {
	return nil, 0, nil
}
func (s *stubItemRepo) Update(_ context.Context, id int64, _ ports.PlanItemPatch) (domain.PlanItem, error) {
	s.updateID = id
	return domain.PlanItem{ID: id}, nil
}

func ownedSvc(items *stubItemRepo) *PlanItemService {
	return &PlanItemService{
		Items: items,
		Goals: stubGoalRepo{goal: domain.Goal{ID: 10, PlanID: 5}},
		Plans: stubPlanRepo{plan: domain.Plan{ID: 5, CreatedBy: 99}},
	}
}

func TestPlanItemCreateDefaultsTypeAndStatus(t *testing.T) {
	items := &stubItemRepo{}
	svc := ownedSvc(items)
	_, err := svc.Create(context.Background(), 99, 5, 10, ports.PlanItemInput{Name: "Run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items.created.ItemType != defaultItemType || items.created.Status != defaultItemStatus {
		t.Errorf("defaults not applied: type=%q status=%q", items.created.ItemType, items.created.Status)
	}
}

func TestPlanItemCreateForbiddenForNonOwner(t *testing.T) {
	svc := ownedSvc(&stubItemRepo{})
	_, err := svc.Create(context.Background(), 1, 5, 10, ports.PlanItemInput{Name: "Run"})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPlanItemCreateGoalNotInPlan(t *testing.T) {
	svc := &PlanItemService{
		Items: &stubItemRepo{},
		Goals: stubGoalRepo{goal: domain.Goal{ID: 10, PlanID: 999}},
		Plans: stubPlanRepo{plan: domain.Plan{ID: 5, CreatedBy: 99}},
	}
	_, err := svc.Create(context.Background(), 99, 5, 10, ports.PlanItemInput{Name: "Run"})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPlanItemUpdateRejectsItemFromOtherGoal(t *testing.T) {
	items := &stubItemRepo{getItem: domain.PlanItem{ID: 1, GoalID: 777}}
	svc := ownedSvc(items)
	_, err := svc.Update(context.Background(), 99, 5, 10, 1, ports.PlanItemPatch{})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProgressPercentClamps(t *testing.T) {
	target := 100.0
	current := 250.0
	it := domain.PlanItem{TargetValue: &target, CurrentValue: &current}
	if p := it.ProgressPercent(); p == nil || *p != 100 {
		t.Fatalf("expected clamp to 100, got %v", p)
	}
	neg := -5.0
	it2 := domain.PlanItem{TargetValue: &target, CurrentValue: &neg}
	if p := it2.ProgressPercent(); p == nil || *p != 0 {
		t.Fatalf("expected clamp to 0, got %v", p)
	}
	it3 := domain.PlanItem{CurrentValue: &current}
	if it3.ProgressPercent() != nil {
		t.Fatalf("expected nil when target missing")
	}
}
