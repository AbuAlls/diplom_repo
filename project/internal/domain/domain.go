package domain

import "time"

type DocStatus string

const (
	DocUploaded   DocStatus = "UPLOADED"
	DocProcessing DocStatus = "PROCESSING"
	DocReady      DocStatus = "READY"
	DocFailed     DocStatus = "FAILED"
)

type StepName string

const (
	StepOCR       StepName = "OCR"
	StepLLM       StepName = "LLM"
	StepNormalize StepName = "NORMALIZE"
)

type Event struct {
	EventID    string    `json:"event_id"`
	OccurredAt time.Time `json:"occurred_at"`
	UserID     string    `json:"user_id"`
	DocumentID string    `json:"document_id"`
	JobID      string    `json:"job_id"`
	Type       string    `json:"type"` // jobs.created, jobs.step.completed, ...
	Step       *StepName `json:"step,omitempty"`
	Attempt    *int      `json:"attempt,omitempty"`
	Data       any       `json:"data,omitempty"`
}
