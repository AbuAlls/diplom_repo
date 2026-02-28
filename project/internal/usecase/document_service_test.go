package usecase

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type docRepoMock struct{}

func (docRepoMock) Create(ctx context.Context, ownerID, filename, mime, checksum, objectKey string, size int64) (string, error) {
	return "doc-1", nil
}
func (docRepoMock) Get(ctx context.Context, ownerID, docID string) (ports.DocumentDTO, error) {
	return ports.DocumentDTO{}, nil
}
func (docRepoMock) List(ctx context.Context, ownerID string, limit int, cursor string, status *domain.DocStatus) ([]ports.DocumentDTO, string, error) {
	return nil, "", nil
}
func (docRepoMock) UpdateStatus(ctx context.Context, docID string, status domain.DocStatus) error {
	return nil
}

type jobRepoMock struct{}

func (jobRepoMock) Create(ctx context.Context, ownerID, docID string, pipelineVersion int) (string, error) {
	return "job-1", nil
}
func (jobRepoMock) GetByOwner(ctx context.Context, ownerID, jobID string) (ports.JobDTO, error) {
	return ports.JobDTO{}, nil
}
func (jobRepoMock) GetByID(ctx context.Context, jobID string) (ports.JobDTO, error) {
	return ports.JobDTO{}, nil
}
func (jobRepoMock) UpsertStep(ctx context.Context, jobID string, step domain.StepName) error {
	return nil
}
func (jobRepoMock) MarkStepRunning(ctx context.Context, jobID string, step domain.StepName) (int, error) {
	return 1, nil
}
func (jobRepoMock) MarkStepCompleted(ctx context.Context, jobID string, step domain.StepName) error {
	return nil
}
func (jobRepoMock) MarkStepFailed(ctx context.Context, jobID string, step domain.StepName, errMsg string) (int, error) {
	return 1, nil
}
func (jobRepoMock) MarkJobRunning(ctx context.Context, jobID string) error   { return nil }
func (jobRepoMock) MarkJobCompleted(ctx context.Context, jobID string) error { return nil }
func (jobRepoMock) MarkJobFailed(ctx context.Context, jobID string) error    { return nil }
func (jobRepoMock) ListCreatedJobs(ctx context.Context, limit int) ([]ports.JobDTO, error) {
	return nil, nil
}

type storeMock struct{}

func (storeMock) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := io.ReadAll(r)
	return err
}
func (storeMock) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}
func (storeMock) PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "", nil
}

type brokerMock struct{ last domain.Event }

func (b *brokerMock) Publish(ctx context.Context, subject string, evt domain.Event) error {
	b.last = evt
	return nil
}
func (b *brokerMock) PublishUI(ctx context.Context, subject string, evt domain.Event) error {
	return nil
}
func (b *brokerMock) Subscribe(ctx context.Context, subject, durable, queue string, handler func(domain.Event) error) error {
	return nil
}

func TestUploadPublishesCreatedEvent(t *testing.T) {
	br := &brokerMock{}
	svc := &DocumentService{
		Docs:   docRepoMock{},
		Jobs:   jobRepoMock{},
		Store:  storeMock{},
		Broker: br,
	}
	_, _, err := svc.Upload(context.Background(), "owner-1", "../a.txt", "text/plain", bytes.NewBufferString("hello"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if br.last.Type != "jobs.created" {
		t.Fatalf("unexpected event type: %s", br.last.Type)
	}
}
