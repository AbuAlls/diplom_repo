package pgcore

import (
	"context"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type PlanRepo struct{ Store *Store }

func NewPlanRepo(store *Store) *PlanRepo { return &PlanRepo{Store: store} }

func (r *PlanRepo) Create(ctx context.Context, ownerID int64, name string, description *string, status string) (domain.Plan, error) {
	const q = `
insert into plans (name, description, created_by, status)
values ($1, $2, $3, $4)
returning id, name, description, created_by, status, created_at, updated_at`
	var p domain.Plan
	err := r.Store.Pool.QueryRow(ctx, q, name, description, ownerID, status).
		Scan(&p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *PlanRepo) GetByID(ctx context.Context, id int64) (domain.Plan, error) {
	const q = `select id, name, description, created_by, status, created_at, updated_at from plans where id = $1`
	var p domain.Plan
	err := r.Store.Pool.QueryRow(ctx, q, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	return p, mapErr(err)
}

func (r *PlanRepo) ListByOwner(ctx context.Context, ownerID int64, offset, limit int) ([]domain.Plan, int, error) {
	var total int
	if err := r.Store.Pool.QueryRow(ctx, `select count(*) from plans where created_by = $1`, ownerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	const q = `
select id, name, description, created_by, status, created_at, updated_at
from plans
where created_by = $1
order by created_at desc, id desc
limit $2 offset $3`
	rows, err := r.Store.Pool.Query(ctx, q, ownerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Plan, 0, limit)
	for rows.Next() {
		var p domain.Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *PlanRepo) ListByOwners(ctx context.Context, ownerIDs []int64, offset, limit int) ([]domain.Plan, int, error) {
	if len(ownerIDs) == 0 {
		return []domain.Plan{}, 0, nil
	}
	var total int
	if err := r.Store.Pool.QueryRow(ctx, `select count(*) from plans where created_by = any($1)`, ownerIDs).Scan(&total); err != nil {
		return nil, 0, err
	}
	const q = `
select id, name, description, created_by, status, created_at, updated_at
from plans
where created_by = any($1)
order by created_at desc, id desc
limit $2 offset $3`
	rows, err := r.Store.Pool.Query(ctx, q, ownerIDs, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Plan, 0, limit)
	for rows.Next() {
		var p domain.Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedBy, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

var _ ports.PlanRepo = (*PlanRepo)(nil)
