package pgcore

import (
	"context"
	"errors"
	"fmt"

	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5"
)

type FolderRepo struct{ Store *Store }

func NewFolderRepo(store *Store) *FolderRepo { return &FolderRepo{Store: store} }

// FindOrCreateItemFolder returns the id of the system folder dedicated to a plan
// item's documents, creating it on first use.
func (r *FolderRepo) FindOrCreateItemFolder(ctx context.Context, planItemID, ownerID int64) (int64, error) {
	const sel = `select id from folders where plan_item_id = $1 and is_system = true order by id asc limit 1`
	var id int64
	err := r.Store.Pool.QueryRow(ctx, sel, planItemID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	const ins = `insert into folders (name, created_by, is_system, plan_item_id)
		values ($1, $2, true, $3) returning id`
	name := fmt.Sprintf("Plan item %d documents", planItemID)
	if err := r.Store.Pool.QueryRow(ctx, ins, name, ownerID, planItemID).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

var _ ports.FolderRepo = (*FolderRepo)(nil)
