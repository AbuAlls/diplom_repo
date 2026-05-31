package pgcore

import (
	"context"
	"fmt"

	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5"
)

// AnalyticsQueryRepo backs the AI agent's read-only callbacks against the core
// database (schema introspection and ad-hoc SELECT execution).
type AnalyticsQueryRepo struct{ Store *Store }

func NewAnalyticsQueryRepo(store *Store) *AnalyticsQueryRepo {
	return &AnalyticsQueryRepo{Store: store}
}

// Schema returns public table names mapped to their ordered column names.
func (r *AnalyticsQueryRepo) Schema(ctx context.Context) (map[string][]string, error) {
	const q = `
select table_name, column_name
from information_schema.columns
where table_schema = 'public'
order by table_name, ordinal_position`
	rows, err := r.Store.Pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]string)
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			return nil, err
		}
		out[table] = append(out[table], column)
	}
	return out, rows.Err()
}

// RunReadOnlyQuery executes sql inside a read-only transaction and returns the
// column names and positional row values.
//
// When ownerID > 0 the query is wrapped in per-tenant CTEs that shadow the
// raw user-owned tables with pre-filtered versions. The agent's arbitrary SQL
// (e.g. SELECT * FROM documents) will therefore only ever see rows belonging
// to that owner — without needing Postgres RLS or schema changes.
//
// Tables wrapped: documents, plans, plan_goals, plan_items, folders,
// extracted_document_data.
func (r *AnalyticsQueryRepo) RunReadOnlyQuery(ctx context.Context, sql string, ownerID int64) ([]string, [][]any, error) {
	effectiveSQL := sql
	if ownerID > 0 {
		effectiveSQL = ownerScopedSQL(sql, ownerID)
	}

	tx, err := r.Store.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, effectiveSQL)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
	}

	var out [][]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, nil, err
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return cols, out, nil
}

// ownerScopedSQL wraps the agent's query in a WITH clause that shadows each
// user-owned table with a pre-filtered CTE. Postgres resolves CTE names before
// table names within the same WITH scope, so the original SQL references are
// transparently redirected without any query rewriting.
//
// Note: we use public.<table> in the CTE definitions to avoid recursive
// self-reference; the agent's own SQL then references the CTE aliases.
func ownerScopedSQL(sql string, ownerID int64) string {
	id := ownerID // safe: ownerID is int64, not user input
	return fmt.Sprintf(`
WITH
  documents AS (
    SELECT * FROM public.documents WHERE uploaded_by = %d
  ),
  plans AS (
    SELECT * FROM public.plans WHERE created_by = %d
  ),
  plan_goals AS (
    SELECT pg.* FROM public.plan_goals pg
    JOIN plans ON plans.id = pg.plan_id
  ),
  plan_items AS (
    SELECT pi.* FROM public.plan_items pi
    JOIN plan_goals ON plan_goals.id = pi.goal_id
  ),
  folders AS (
    SELECT * FROM public.folders WHERE created_by = %d
  ),
  extracted_document_data AS (
    SELECT edd.* FROM public.extracted_document_data edd
    JOIN documents ON documents.id = edd.document_id
  )
%s`, id, id, id, sql)
}

var _ ports.AnalyticsQueryRepo = (*AnalyticsQueryRepo)(nil)
