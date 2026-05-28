package pgcore

import (
	"context"
	"fmt"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5"
)

type PlanItemRepo struct{ Store *Store }

func NewPlanItemRepo(store *Store) *PlanItemRepo { return &PlanItemRepo{Store: store} }

const itemSelectCols = `i.id, i.goal_id, g.name, i.name, i.description, i.item_type, i.status,
	i.target_value::float8, i.current_value::float8, i.unit, i.sort_order, i.created_at, i.updated_at`

func scanItem(row pgx.Row) (domain.PlanItem, error) {
	var it domain.PlanItem
	err := row.Scan(&it.ID, &it.GoalID, &it.GoalName, &it.Name, &it.Description, &it.ItemType,
		&it.Status, &it.TargetValue, &it.CurrentValue, &it.Unit, &it.SortOrder, &it.CreatedAt, &it.UpdatedAt)
	return it, err
}

func (r *PlanItemRepo) Create(ctx context.Context, goalID int64, in ports.PlanItemInput) (domain.PlanItem, error) {
	const q = `
with ins as (
	insert into plan_items (goal_id, name, description, item_type, status, target_value, current_value, unit, sort_order)
	values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	returning *
)
select ` + itemSelectCols + `
from ins i join plan_goals g on g.id = i.goal_id`
	row := r.Store.Pool.QueryRow(ctx, q, goalID, in.Name, in.Description, in.ItemType, in.Status,
		in.TargetValue, in.CurrentValue, in.Unit, in.SortOrder)
	return scanItem(row)
}

func (r *PlanItemRepo) GetByID(ctx context.Context, id int64) (domain.PlanItem, error) {
	const q = `select ` + itemSelectCols + `
from plan_items i join plan_goals g on g.id = i.goal_id
where i.id = $1`
	it, err := scanItem(r.Store.Pool.QueryRow(ctx, q, id))
	return it, mapErr(err)
}

func (r *PlanItemRepo) ListByGoal(ctx context.Context, goalID int64, offset, limit int) ([]domain.PlanItem, int, error) {
	var total int
	if err := r.Store.Pool.QueryRow(ctx, `select count(*) from plan_items where goal_id = $1`, goalID).Scan(&total); err != nil {
		return nil, 0, err
	}
	const q = `select ` + itemSelectCols + `
from plan_items i join plan_goals g on g.id = i.goal_id
where i.goal_id = $1
order by i.sort_order asc, i.id asc
limit $2 offset $3`
	rows, err := r.Store.Pool.Query(ctx, q, goalID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.PlanItem, 0, limit)
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (r *PlanItemRepo) Update(ctx context.Context, id int64, patch ports.PlanItemPatch) (domain.PlanItem, error) {
	sets := []string{}
	args := []any{}
	add := func(col string, val any) {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if patch.Name != nil {
		add("name", *patch.Name)
	}
	if patch.Description != nil {
		add("description", *patch.Description)
	}
	if patch.SortOrder != nil {
		add("sort_order", *patch.SortOrder)
	}
	if patch.ItemType != nil {
		add("item_type", *patch.ItemType)
	}
	if patch.Status != nil {
		add("status", *patch.Status)
	}
	if patch.TargetValue != nil {
		add("target_value", *patch.TargetValue)
	}
	if patch.CurrentValue != nil {
		add("current_value", *patch.CurrentValue)
	}
	if patch.Unit != nil {
		add("unit", *patch.Unit)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	q := fmt.Sprintf(`
with upd as (
	update plan_items set %s where id = $%d returning *
)
select %s
from upd i join plan_goals g on g.id = i.goal_id`, strings.Join(sets, ", "), len(args), itemSelectCols)
	it, err := scanItem(r.Store.Pool.QueryRow(ctx, q, args...))
	return it, mapErr(err)
}

var _ ports.PlanItemRepo = (*PlanItemRepo)(nil)
