package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

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
	Type       string    `json:"type"`
	UserID     string    `json:"user_id"`
	DocumentID string    `json:"document_id"`
	JobID      string    `json:"job_id"`
	Step       *StepName `json:"step,omitempty"`
	Attempt    *int      `json:"attempt,omitempty"`
	Data       any       `json:"data,omitempty"`
}

func NewEventID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}
