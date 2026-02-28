package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type DocumentService struct {
	Docs   ports.DocumentRepo
	Jobs   ports.JobRepo
	Store  ports.ObjectStore
	Broker ports.Broker
}

func (s *DocumentService) Upload(ctx context.Context, ownerID, filename, mime string, r io.Reader, size int64) (docID, jobID string, err error) {
	safeName := sanitizeFilename(filename)
	h := sha256.New()
	tee := io.TeeReader(r, h)
	key := fmt.Sprintf("%s/%s_%s", ownerID, time.Now().UTC().Format("20060102T150405Z"), safeName)

	if err := s.Store.Put(ctx, key, tee, size, mime); err != nil {
		return "", "", err
	}
	checksum := hex.EncodeToString(h.Sum(nil))

	docID, err = s.Docs.Create(ctx, ownerID, safeName, mime, checksum, key, size)
	if err != nil {
		return "", "", err
	}
	jobID, err = s.Jobs.Create(ctx, ownerID, docID, 1)
	if err != nil {
		return "", "", err
	}

	evt := domain.Event{
		EventID:    domain.NewEventID(),
		OccurredAt: time.Now().UTC(),
		Type:       "jobs.created",
		UserID:     ownerID,
		DocumentID: docID,
		JobID:      jobID,
		Data: map[string]any{
			"pipeline_version": 1,
		},
	}
	if err := s.Broker.Publish(ctx, "jobs.created", evt); err != nil {
		return "", "", err
	}
	_ = s.Broker.PublishUI(ctx, "ui.jobs."+jobID, evt)
	return docID, jobID, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	if name == "." || name == "" || name == "/" {
		return "file.bin"
	}
	return name
}
