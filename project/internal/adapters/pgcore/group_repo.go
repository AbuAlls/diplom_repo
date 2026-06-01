package pgcore

import (
	"context"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

// GroupRepo is the Postgres-backed corporate-account group store. It uses the
// existing `groups` and `user_group_relations` tables (see migrations
// 000001 and 000003).
type GroupRepo struct{ Store *Store }

func NewGroupRepo(store *Store) *GroupRepo { return &GroupRepo{Store: store} }

const groupCols = `id, name, description, role, created_by, created_at, updated_at`

func (r *GroupRepo) Create(ctx context.Context, createdBy int64, name, description string) (domain.Group, error) {
	const q = `
insert into groups (name, description, role, created_by)
values ($1, $2, 'corporate', $3)
returning ` + groupCols
	var g domain.Group
	err := r.Store.Pool.QueryRow(ctx, q, name, description, createdBy).
		Scan(&g.ID, &g.Name, &g.Description, &g.Role, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return domain.Group{}, err
	}
	// The creator is automatically the first member.
	if err := r.AddMember(ctx, g.ID, createdBy); err != nil {
		return domain.Group{}, err
	}
	return g, nil
}

func (r *GroupRepo) GetByID(ctx context.Context, id int64) (domain.Group, error) {
	const q = `select ` + groupCols + ` from groups where id = $1`
	var g domain.Group
	err := r.Store.Pool.QueryRow(ctx, q, id).
		Scan(&g.ID, &g.Name, &g.Description, &g.Role, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt)
	return g, mapErr(err)
}

func (r *GroupRepo) ListByMember(ctx context.Context, userID int64, offset, limit int) ([]domain.Group, int, error) {
	var total int
	const countQ = `select count(*) from user_group_relations where user_id = $1`
	if err := r.Store.Pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	const q = `
select g.id, g.name, g.description, g.role, g.created_by, g.created_at, g.updated_at
from groups g
join user_group_relations ugr on ugr.group_id = g.id
where ugr.user_id = $1
order by g.created_at desc, g.id desc
limit $2 offset $3`
	rows, err := r.Store.Pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Group, 0, limit)
	for rows.Next() {
		var g domain.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Role, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, g)
	}
	return out, total, rows.Err()
}

func (r *GroupRepo) AddMember(ctx context.Context, groupID, userID int64) error {
	const q = `
insert into user_group_relations (group_id, user_id)
values ($1, $2)
on conflict (group_id, user_id) do nothing`
	_, err := r.Store.Pool.Exec(ctx, q, groupID, userID)
	return err
}

func (r *GroupRepo) RemoveMember(ctx context.Context, groupID, userID int64) error {
	const q = `delete from user_group_relations where group_id = $1 and user_id = $2`
	_, err := r.Store.Pool.Exec(ctx, q, groupID, userID)
	return err
}

func (r *GroupRepo) ListMembers(ctx context.Context, groupID int64) ([]domain.GroupMember, error) {
	const q = `
select u.id, ugr.group_id, u.email, u.full_name, ugr.id
from user_group_relations ugr
join users u on u.id = ugr.user_id
where ugr.group_id = $1
order by u.full_name, u.id`
	rows, err := r.Store.Pool.Query(ctx, q, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.GroupMember
	for rows.Next() {
		var m domain.GroupMember
		var ignore int64
		if err := rows.Scan(&m.UserID, &m.GroupID, &m.Email, &m.FullName, &ignore); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *GroupRepo) IsMember(ctx context.Context, groupID, userID int64) (bool, error) {
	const q = `select exists(select 1 from user_group_relations where group_id = $1 and user_id = $2)`
	var ok bool
	err := r.Store.Pool.QueryRow(ctx, q, groupID, userID).Scan(&ok)
	return ok, err
}

// CoMemberIDs returns userID plus every user that shares at least one group with
// it. The result always contains userID, even when the user has no groups.
func (r *GroupRepo) CoMemberIDs(ctx context.Context, userID int64) ([]int64, error) {
	const q = `
select distinct peer.user_id
from user_group_relations me
join user_group_relations peer on peer.group_id = me.group_id
where me.user_id = $1`
	rows, err := r.Store.Pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{userID}
	seen := map[int64]bool{userID: true}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

var _ ports.GroupRepo = (*GroupRepo)(nil)
