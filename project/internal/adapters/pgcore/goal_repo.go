package pgcore

import (
	"context"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type GoalRepo struct{ Store *Store }

func NewGoalRepo(store *Store) *GoalRepo { return &GoalRepo{Store: store} }

func (r *GoalRepo) Create(ctx context.Context, planID int64, name string, description *string, sortOrder int) (domain.Goal, error) {
	const q = `
insert into plan_goals (plan_id, name, description, sort_order)
values ($1, $2, $3, $4)
returning id, plan_id, name, description, sort_order, created_at, updated_at`
	var g domain.Goal
	err := r.Store.Pool.QueryRow(ctx, q, planID, name, description, sortOrder).
		Scan(&g.ID, &g.PlanID, &g.Name, &g.Description, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

func (r *GoalRepo) GetByID(ctx context.Context, id int64) (domain.Goal, error) {
	const q = `select id, plan_id, name, description, sort_order, created_at, updated_at from plan_goals where id = $1`
	var g domain.Goal
	err := r.Store.Pool.QueryRow(ctx, q, id).
		Scan(&g.ID, &g.PlanID, &g.Name, &g.Description, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt)
	return g, mapErr(err)
}

func (r *GoalRepo) ListByPlan(ctx context.Context, planID int64, offset, limit int) ([]domain.Goal, int, error) {
	var total int
	if err := r.Store.Pool.QueryRow(ctx, `select count(*) from plan_goals where plan_id = $1`, planID).Scan(&total); err != nil {
		return nil, 0, err
	}
	const q = `
select id, plan_id, name, description, sort_order, created_at, updated_at
from plan_goals
where plan_id = $1
order by sort_order asc, id asc
limit $2 offset $3`
	rows, err := r.Store.Pool.Query(ctx, q, planID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Goal, 0, limit)
	for rows.Next() {
		var g domain.Goal
		if err := rows.Scan(&g.ID, &g.PlanID, &g.Name, &g.Description, &g.SortOrder, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

var _ ports.GoalRepo = (*GoalRepo)(nil)
