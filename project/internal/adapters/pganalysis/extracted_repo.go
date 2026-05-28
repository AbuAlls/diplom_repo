package pganalysis

import (
	"context"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"github.com/jackc/pgx/v5"
)

type ExtractedDataRepo struct{ Store *Store }

func NewExtractedDataRepo(store *Store) *ExtractedDataRepo { return &ExtractedDataRepo{Store: store} }

const extractedCols = `id, document_id, recognized_text, structured_json, recognized_category_id,
	confidence_score::float8, processing_status, processed_at, model_version`

func scanExtracted(row pgx.Row) (domain.ExtractedData, error) {
	var e domain.ExtractedData
	err := row.Scan(&e.ID, &e.DocumentID, &e.RecognizedText, &e.StructuredJSON, &e.RecognizedCategoryID,
		&e.ConfidenceScore, &e.ProcessingStatus, &e.ProcessedAt, &e.ModelVersion)
	return e, err
}

func (r *ExtractedDataRepo) Create(ctx context.Context, in ports.ExtractedDataCreate) (domain.ExtractedData, error) {
	const q = `
insert into extracted_document_data
	(document_id, recognized_text, structured_json, recognized_category_id, confidence_score, processing_status, processed_at, model_version)
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning ` + extractedCols
	row := r.Store.Pool.QueryRow(ctx, q, in.DocumentID, in.RecognizedText, []byte(in.StructuredJSON),
		in.RecognizedCategoryID, in.ConfidenceScore, in.ProcessingStatus, in.ProcessedAt, in.ModelVersion)
	return scanExtracted(row)
}

func (r *ExtractedDataRepo) GetByDocumentID(ctx context.Context, documentID int64) (domain.ExtractedData, error) {
	const q = `select ` + extractedCols + ` from extracted_document_data where document_id = $1 order by id desc limit 1`
	e, err := scanExtracted(r.Store.Pool.QueryRow(ctx, q, documentID))
	return e, mapErr(err)
}

func (r *ExtractedDataRepo) ListByDocumentIDs(ctx context.Context, documentIDs []int64) (map[int64]domain.ExtractedData, error) {
	out := make(map[int64]domain.ExtractedData, len(documentIDs))
	if len(documentIDs) == 0 {
		return out, nil
	}
	const q = `select ` + extractedCols + ` from extracted_document_data where document_id = any($1) order by id asc`
	rows, err := r.Store.Pool.Query(ctx, q, documentIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanExtracted(rows)
		if err != nil {
			return nil, err
		}
		out[e.DocumentID] = e // later rows (higher id) win
	}
	return out, rows.Err()
}

func (r *ExtractedDataRepo) UpdateCategory(ctx context.Context, documentID int64, categoryID *int64) error {
	const q = `update extracted_document_data set recognized_category_id = $2 where document_id = $1`
	_, err := r.Store.Pool.Exec(ctx, q, documentID, categoryID)
	return err
}

var _ ports.ExtractedDataRepo = (*ExtractedDataRepo)(nil)
