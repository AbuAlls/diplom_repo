package recognition

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"diplom.com/m/internal/ports"
)

const modelVersion = "mock-v0"

// MockRecognizer is a deterministic stand-in for the OCR/LLM analysis service.
// It produces stable, plausible output so the upload flow and tests are
// exercisable end-to-end without a real model.
type MockRecognizer struct{}

func New() *MockRecognizer { return &MockRecognizer{} }

func (MockRecognizer) Recognize(_ context.Context, in ports.RecognizeInput) (ports.RecognizeResult, error) {
	confidence := 0.95
	org := "ООО Ромашка"
	inn := "7700000000"
	extNo := fmt.Sprintf("DOC-%d", in.DocumentID)
	deadline := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	structured, _ := json.Marshal(map[string]any{
		"mock":      true,
		"source":    in.FileName,
		"mime_type": in.MimeType,
	})

	return ports.RecognizeResult{
		RecognizedText:   fmt.Sprintf("Mock recognized text for %q", in.FileName),
		StructuredJSON:   structured,
		ConfidenceScore:  &confidence,
		ModelVersion:     modelVersion,
		ExternalNumber:   &extNo,
		OrganizationName: &org,
		INN:              &inn,
		Deadlines:        []time.Time{deadline},
		ProductNames:     []string{"Mock product"},
		Quantities:       []int32{1},
	}, nil
}

var _ ports.Recognizer = (*MockRecognizer)(nil)
