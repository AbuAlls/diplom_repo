package usecase

import (
	"context"
	"testing"

	"diplom.com/m/internal/domain"
)

type fakePlanRepo struct {
	created   domain.Plan
	createArg struct {
		ownerID int64
		name    string
		desc    *string
		status  string
	}
	list  []domain.Plan
	total int
}

func (f *fakePlanRepo) Create(_ context.Context, ownerID int64, name string, desc *string, status string) (domain.Plan, error) {
	f.createArg.ownerID = ownerID
	f.createArg.name = name
	f.createArg.desc = desc
	f.createArg.status = status
	f.created = domain.Plan{ID: 1, Name: name, Description: desc, CreatedBy: ownerID, Status: status}
	return f.created, nil
}

func (f *fakePlanRepo) GetByID(_ context.Context, id int64) (domain.Plan, error) {
	return domain.Plan{ID: id, CreatedBy: f.createArg.ownerID}, nil
}

func (f *fakePlanRepo) ListByOwner(_ context.Context, _ int64, _, _ int) ([]domain.Plan, int, error) {
	return f.list, f.total, nil
}

func (f *fakePlanRepo) ListByOwners(_ context.Context, _ []int64, _, _ int) ([]domain.Plan, int, error) {
	return f.list, f.total, nil
}

func TestPlanServiceCreateDefaultsStatus(t *testing.T) {
	repo := &fakePlanRepo{}
	svc := &PlanService{Plans: repo}

	plan, err := svc.Create(context.Background(), 7, CreatePlanInput{Name: "  My Plan  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createArg.name != "My Plan" {
		t.Errorf("name not trimmed: %q", repo.createArg.name)
	}
	if repo.createArg.status != defaultPlanStatus {
		t.Errorf("expected default status %q, got %q", defaultPlanStatus, repo.createArg.status)
	}
	if plan.CreatedBy != 7 {
		t.Errorf("owner not propagated: %d", plan.CreatedBy)
	}
}

func TestPlanServiceCreateRejectsEmptyName(t *testing.T) {
	svc := &PlanService{Plans: &fakePlanRepo{}}
	if _, err := svc.Create(context.Background(), 1, CreatePlanInput{Name: "   "}); err != ErrValidation {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestPlanServiceListPropagates(t *testing.T) {
	repo := &fakePlanRepo{list: []domain.Plan{{ID: 1}, {ID: 2}}, total: 5}
	svc := &PlanService{Plans: repo}
	items, total, err := svc.List(context.Background(), 1, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 || total != 5 {
		t.Errorf("unexpected list result: %d items, total %d", len(items), total)
	}
}
