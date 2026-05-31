package pgcore

import (
	"context"

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
// column names and positional row values. The read-only transaction is a
// hard backstop in addition to the use-case SQL validation.
func (r *AnalyticsQueryRepo) RunReadOnlyQuery(ctx context.Context, sql string) ([]string, [][]any, error) {
	tx, err := r.Store.Pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, sql)
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

var _ ports.AnalyticsQueryRepo = (*AnalyticsQueryRepo)(nil)
