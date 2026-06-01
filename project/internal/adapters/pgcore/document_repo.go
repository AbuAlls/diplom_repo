package pgcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const serializableMaxRetries = 5

type DocumentRepo struct{ Store *Store }

func NewDocumentRepo(store *Store) *DocumentRepo { return &DocumentRepo{Store: store} }

const docSelectCols = `d.id, coalesce(f.plan_item_id, 0), d.title, d.folder_id, d.category_id,
	d.uploaded_by, d.status, d.document_date, d.external_number, d.organization_name, d.inn, d.description,
	d.file_name, d.file_path, d.mime_type, d.file_size,
	d.deadlines, d.personal_data, d.organization_data, d.prices, d.quantities, d.product_names, d.contract_numbers,
	d.created_at, d.updated_at`

func scanDoc(row pgx.Row) (domain.Document, error) {
	var d domain.Document
	err := row.Scan(&d.ID, &d.PlanItemID, &d.Title, &d.FolderID, &d.CategoryID,
		&d.UploadedBy, &d.Status, &d.DocumentDate, &d.ExternalNumber, &d.OrganizationName, &d.INN, &d.Description,
		&d.FileName, &d.FilePath, &d.MimeType, &d.FileSize,
		&d.Deadlines, &d.PersonalData, &d.OrganizationData, &d.Prices, &d.Quantities, &d.ProductNames, &d.ContractNumbers,
		&d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *DocumentRepo) Create(ctx context.Context, in ports.DocumentCreate) (domain.Document, error) {
	const q = `
with ins as (
	insert into documents (title, folder_id, uploaded_by, status, file_name, file_path, mime_type, file_size)
	values ($1, $2, $3, $4, $5, $6, $7, $8)
	returning *
)
select ` + docSelectCols + `
from ins d join folders f on f.id = d.folder_id`
	row := r.Store.Pool.QueryRow(ctx, q, in.Title, in.FolderID, in.UploadedBy, in.Status,
		in.FileName, in.FilePath, in.MimeType, in.FileSize)
	return scanDoc(row)
}

func (r *DocumentRepo) GetByID(ctx context.Context, id int64) (domain.Document, error) {
	const q = `select ` + docSelectCols + `
from documents d join folders f on f.id = d.folder_id
where d.id = $1`
	d, err := scanDoc(r.Store.Pool.QueryRow(ctx, q, id))
	return d, mapErr(err)
}

func (r *DocumentRepo) ListByOwner(ctx context.Context, ownerID int64, planItemID *int64, offset, limit int) ([]domain.Document, int, error) {
	where := "d.uploaded_by = $1"
	args := []any{ownerID}
	if planItemID != nil {
		args = append(args, *planItemID)
		where += fmt.Sprintf(" and f.plan_item_id = $%d", len(args))
	}

	countQ := "select count(*) from documents d join folders f on f.id = d.folder_id where " + where
	var total int
	if err := r.Store.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf(`select %s
from documents d join folders f on f.id = d.folder_id
where %s
order by d.created_at desc, d.id desc
limit $%d offset $%d`, docSelectCols, where, len(args)-1, len(args))
	rows, err := r.Store.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Document, 0, limit)
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (r *DocumentRepo) ListByOwners(ctx context.Context, ownerIDs []int64, planItemID *int64, offset, limit int) ([]domain.Document, int, error) {
	if len(ownerIDs) == 0 {
		return []domain.Document{}, 0, nil
	}
	where := "d.uploaded_by = any($1)"
	args := []any{ownerIDs}
	if planItemID != nil {
		args = append(args, *planItemID)
		where += fmt.Sprintf(" and f.plan_item_id = $%d", len(args))
	}

	countQ := "select count(*) from documents d join folders f on f.id = d.folder_id where " + where
	var total int
	if err := r.Store.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	q := fmt.Sprintf(`select %s
from documents d join folders f on f.id = d.folder_id
where %s
order by d.created_at desc, d.id desc
limit $%d offset $%d`, docSelectCols, where, len(args)-1, len(args))
	rows, err := r.Store.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]domain.Document, 0, limit)
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

func (r *DocumentRepo) UpdateFields(ctx context.Context, id int64, patch ports.DocumentPatch) (domain.Document, error) {
	sets := []string{}
	args := []any{}
	add := func(col string, val any) {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if patch.DocumentDate != nil {
		add("document_date", *patch.DocumentDate)
	}
	if patch.ExternalNumber != nil {
		add("external_number", *patch.ExternalNumber)
	}
	if patch.OrganizationName != nil {
		add("organization_name", *patch.OrganizationName)
	}
	if patch.INN != nil {
		add("inn", *patch.INN)
	}
	if patch.Description != nil {
		add("description", *patch.Description)
	}
	if patch.Deadlines != nil {
		add("deadlines", patch.Deadlines)
	}
	if patch.PersonalData != nil {
		add("personal_data", patch.PersonalData)
	}
	if patch.OrganizationData != nil {
		add("organization_data", patch.OrganizationData)
	}
	if patch.Prices != nil {
		add("prices", patch.Prices)
	}
	if patch.Quantities != nil {
		add("quantities", patch.Quantities)
	}
	if patch.ProductNames != nil {
		add("product_names", patch.ProductNames)
	}
	if patch.ContractNumbers != nil {
		add("contract_numbers", patch.ContractNumbers)
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	q := fmt.Sprintf(`
with upd as (
	update documents set %s where id = $%d returning *
)
select %s
from upd d join folders f on f.id = d.folder_id`, strings.Join(sets, ", "), len(args), docSelectCols)
	d, err := scanDoc(r.Store.Pool.QueryRow(ctx, q, args...))
	return d, mapErr(err)
}

func (r *DocumentRepo) UpdateStatus(ctx context.Context, id int64, status string) (domain.Document, error) {
	const q = `
with upd as (
	update documents set status = $2, updated_at = now() where id = $1 returning *
)
select ` + docSelectCols + `
from upd d join folders f on f.id = d.folder_id`
	d, err := scanDoc(r.Store.Pool.QueryRow(ctx, q, id, status))
	return d, mapErr(err)
}

// UpdateStatusSerializable transitions a document's status inside a
// SERIALIZABLE transaction, retrying when Postgres aborts with a serialization
// failure (SQLSTATE 40001). This is the safe path for the asynchronous worker
// and for corporate-account members who may act on the same shared document
// concurrently: SERIALIZABLE guarantees the read-decide-write cycle behaves as
// if transactions ran one at a time, so two racing transitions cannot both
// succeed on stale state.
//
// When expectFrom is non-empty the transition is conditional: the update only
// applies if the current status is one of expectFrom, otherwise ErrConflict is
// returned (without retrying — the precondition genuinely failed).
func (r *DocumentRepo) UpdateStatusSerializable(ctx context.Context, id int64, status string, expectFrom ...string) (domain.Document, error) {
	var lastErr error
	for attempt := 0; attempt < serializableMaxRetries; attempt++ {
		doc, err := r.updateStatusSerializableOnce(ctx, id, status, expectFrom)
		if err == nil {
			return doc, nil
		}
		if isSerializationFailure(err) {
			lastErr = err
			// Brief backoff with jitter-free linear growth; contention here is rare.
			select {
			case <-ctx.Done():
				return domain.Document{}, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 5 * time.Millisecond):
			}
			continue
		}
		return domain.Document{}, err
	}
	return domain.Document{}, lastErr
}

func (r *DocumentRepo) updateStatusSerializableOnce(ctx context.Context, id int64, status string, expectFrom []string) (domain.Document, error) {
	tx, err := r.Store.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.Document{}, err
	}
	defer tx.Rollback(ctx)

	// Read current status inside the serializable snapshot.
	var current string
	if err := tx.QueryRow(ctx, `select status from documents where id = $1`, id).Scan(&current); err != nil {
		return domain.Document{}, mapErr(err)
	}
	if len(expectFrom) > 0 && !contains(expectFrom, current) {
		return domain.Document{}, ports.ErrConflict
	}

	const upd = `
with upd as (
	update documents set status = $2, updated_at = now() where id = $1 returning *
)
select ` + docSelectCols + `
from upd d join folders f on f.id = d.folder_id`
	doc, err := scanDoc(tx.QueryRow(ctx, upd, id, status))
	if err != nil {
		return domain.Document{}, mapErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Document{}, err
	}
	return doc, nil
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// isSerializationFailure reports whether err is a Postgres serialization failure
// (40001) or deadlock (40P01), both of which are safe to retry.
func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001" || pgErr.Code == "40P01"
	}
	return false
}

func (r *DocumentRepo) CountByPlanItem(ctx context.Context, planItemID int64) (int, error) {
	const q = `select count(*) from documents d join folders f on f.id = d.folder_id where f.plan_item_id = $1`
	var n int
	err := r.Store.Pool.QueryRow(ctx, q, planItemID).Scan(&n)
	return n, err
}

func (r *DocumentRepo) LatestDocIDByPlanItem(ctx context.Context, planItemID int64) (*int64, error) {
	const q = `select d.id from documents d join folders f on f.id = d.folder_id
where f.plan_item_id = $1 order by d.created_at desc, d.id desc limit 1`
	var id int64
	err := r.Store.Pool.QueryRow(ctx, q, planItemID).Scan(&id)
	if err != nil {
		if mapErr(err) == ports.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

var _ ports.DocumentRepo = (*DocumentRepo)(nil)
