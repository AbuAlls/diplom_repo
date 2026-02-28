package ports

import (
	"context"
	"io"
	"time"

	"diplom.com/m/internal/domain"
)

type DocumentRepo interface {
	Create(ctx context.Context, ownerID, filename, mime, checksum, objectKey string, size int64) (docID string, err error)
	Get(ctx context.Context, ownerID, docID string) (DocumentDTO, error)
	List(ctx context.Context, ownerID string, limit int, cursor string, status *domain.DocStatus) ([]DocumentDTO, string, error)
	UpdateStatus(ctx context.Context, docID string, status domain.DocStatus) error
}

type JobRepo interface {
	Create(ctx context.Context, ownerID, docID string, pipelineVersion int) (jobID string, err error)
	Get(ctx context.Context, jobID string) (JobDTO, error)
	UpsertStep(ctx context.Context, jobID string, step domain.StepName) error
	MarkStepRunning(ctx context.Context, jobID string, step domain.StepName) (attempt int, err error)
	MarkStepCompleted(ctx context.Context, jobID string, step domain.StepName) error
	MarkStepFailed(ctx context.Context, jobID string, step domain.StepName, errMsg string) (attempt int, err error)
	MarkJobRunning(ctx context.Context, jobID string) error
	MarkJobCompleted(ctx context.Context, jobID string) error
	MarkJobFailed(ctx context.Context, jobID string) error
}

type AnalysisRepo interface {
	SaveExtraction(ctx context.Context, ownerID, docID string, fields map[string]AnalysisField) error
}

type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	PresignGetURL(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type Broker interface {
	Publish(ctx context.Context, subject string, evt domain.Event) error
	PublishUI(ctx context.Context, subject string, evt domain.Event) error // plain NATS for SSE
	Subscribe(ctx context.Context, subject, durable, queue string, handler func(domain.Event) error) error
}

type OCRClient interface {
	ExtractText(ctx context.Context, doc io.Reader) (string, error)
}

type LLMClient interface {
	Analyze(ctx context.Context, text string) (map[string]AnalysisField, error)
}

type AnalysisField struct {
	Value      string
	Confidence float32
	Meta       map[string]any
}

// DTOs упрощённо
type DocumentDTO struct {
	ID        string
	OwnerID   string
	Filename  string
	Mime      string
	Size      int64
	Checksum  string
	ObjectKey string
	Status    domain.DocStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type JobDTO struct {
	ID         string
	DocumentID string
	OwnerID    string
	Status     string
}
