package pganalysis

import (
	"context"
	"encoding/json"

	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Repo, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Repo{Pool: pool}, nil
}

func (r *Repo) Close() { r.Pool.Close() }

func (r *Repo) SaveExtraction(ctx context.Context, ownerID, docID string, fields map[string]ports.AnalysisField) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// idempotent by document+owner: replace previous extraction snapshot
	_, err = tx.Exec(ctx, `delete from extractions where owner_id = $1 and document_id = $2`, ownerID, docID)
	if err != nil {
		return err
	}

	var extractionID string
	err = tx.QueryRow(ctx, `insert into extractions (owner_id, document_id) values ($1, $2) returning id::text`, ownerID, docID).Scan(&extractionID)
	if err != nil {
		return err
	}

	for key, field := range fields {
		meta, _ := json.Marshal(field.Meta)
		if _, err = tx.Exec(ctx, `insert into extracted_fields (extraction_id, key, value, confidence, meta) values ($1, $2, $3, $4, $5)`, extractionID, key, field.Value, field.Confidence, meta); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repo) GetExtraction(ctx context.Context, ownerID, docID string) (map[string]ports.AnalysisField, error) {
	const q = `
select ef.key, ef.value, coalesce(ef.confidence,0), ef.meta
from extractions e
join extracted_fields ef on ef.extraction_id = e.id
where e.owner_id = $1 and e.document_id = $2`
	rows, err := r.Pool.Query(ctx, q, ownerID, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ports.AnalysisField{}
	for rows.Next() {
		var key, value string
		var confidence float32
		var metaBytes []byte
		if err := rows.Scan(&key, &value, &confidence, &metaBytes); err != nil {
			return nil, err
		}
		meta := map[string]any{}
		_ = json.Unmarshal(metaBytes, &meta)
		out[key] = ports.AnalysisField{Value: value, Confidence: confidence, Meta: meta}
	}
	return out, rows.Err()
}

var _ ports.AnalysisRepo = (*Repo)(nil)
