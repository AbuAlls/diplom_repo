package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	"diplom.com/m/internal/ports"

	"diplom.com/m/internal/domain"
)

type DocumentService struct {
	Docs   ports.DocumentRepo
	Jobs   ports.JobRepo
	Store  ports.ObjectStore
	Broker ports.Broker
}

func (s *DocumentService) Upload(ctx context.Context, ownerID, filename, mime string, r io.Reader, size int64) (docID, jobID string, err error) {
	// 1) checksum (streaming)
	h := sha256.New()
	tee := io.TeeReader(r, h)

	// 2) object key
	tmpKey := ownerID + "/" + time.Now().UTC().Format("20060102T150405Z") + "_" + filename

	// 3) upload to object store
	if err := s.Store.Put(ctx, tmpKey, tee, size, mime); err != nil {
		return "", "", err
	}
	checksum := hex.EncodeToString(h.Sum(nil))

	// 4) create document row
	docID, err = s.Docs.Create(ctx, ownerID, filename, mime, checksum, tmpKey, size)
	if err != nil {
		return "", "", err
	}

	// 5) create job
	jobID, err = s.Jobs.Create(ctx, ownerID, docID, 1)
	if err != nil {
		return "", "", err
	}

	// 6) publish job.created
	evt := domain.Event{
		EventID:    newUUID(),
		OccurredAt: time.Now().UTC(),
		UserID:     ownerID,
		DocumentID: docID,
		JobID:      jobID,
		Type:       "jobs.created",
		Data: map[string]any{
			"pipeline_version": 1,
		},
	}
	_ = s.Broker.Publish(ctx, "jobs.created", evt)
	_ = s.Broker.PublishUI(ctx, "ui.jobs."+jobID, evt)

	return docID, jobID, nil
}

// замените на нормальный uuid generator
func newUUID() string { return time.Now().UTC().Format("20060102150405.000000000") }
