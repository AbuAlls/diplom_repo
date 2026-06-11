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
//
// For demo purposes it recognizes a catalog of known example files by name (see
// examples.go) and returns individualized, scripted analytics for each — so
// uploading e.g. "contract_orbita.pdf" yields different fields, summary, checks
// and risks than "invoice_alfa.pdf". Unknown files get a consistent generic
// result so the structured_json shape is always the same.
type MockRecognizer struct{}

func New() *MockRecognizer { return &MockRecognizer{} }

func (MockRecognizer) Recognize(_ context.Context, in ports.RecognizeInput) (ports.RecognizeResult, error) {
	if ex, ok := lookupExample(in.FileName); ok {
		return exampleResult(in, ex), nil
	}
	return genericResult(in), nil
}

// exampleResult builds the recognition result for a known example file.
func exampleResult(in ports.RecognizeInput, ex exampleDoc) ports.RecognizeResult {
	confidence := ex.confidence
	docDate := ex.documentDate
	res := ports.RecognizeResult{
		RecognizedText:  ex.recognizedText,
		StructuredJSON:  structuredJSON(in.FileName, ex.summary, ex.checks, ex.risks),
		ConfidenceScore: &confidence,
		ModelVersion:    modelVersion,
		DocumentDate:    &docDate,
		ExternalNumber:  strPtr(ex.extNo),
		OrganizationName: func() *string {
			if ex.org == "" {
				return nil
			}
			return strPtr(ex.org)
		}(),
		INN: func() *string {
			if ex.inn == "" {
				return nil
			}
			return strPtr(ex.inn)
		}(),
		Deadlines:       ex.deadlines,
		Prices:          ex.prices,
		Quantities:      ex.quantities,
		ProductNames:    ex.productNames,
		ContractNumbers: ex.contractNums,
	}
	return res
}

// genericResult preserves the original deterministic behavior for any file not
// in the example catalog, including an empty checks/risks structure so the
// structured_json shape is consistent with the example documents.
func genericResult(in ports.RecognizeInput) ports.RecognizeResult {
	confidence := 0.95
	org := "ООО Ромашка"
	inn := "7700000000"
	extNo := fmt.Sprintf("DOC-%d", in.DocumentID)
	deadline := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	summary := fmt.Sprintf("Документ %q обработан. Структурированные данные извлечены автоматически.", in.FileName)

	return ports.RecognizeResult{
		RecognizedText:   fmt.Sprintf("Mock recognized text for %q", in.FileName),
		StructuredJSON:   structuredJSON(in.FileName, summary, nil, nil),
		ConfidenceScore:  &confidence,
		ModelVersion:     modelVersion,
		ExternalNumber:   &extNo,
		OrganizationName: &org,
		INN:              &inn,
		Deadlines:        []time.Time{deadline},
		ProductNames:     []string{"Mock product"},
		Quantities:       []int32{1},
	}
}

// structuredJSON builds the structured_json payload surfaced to the frontend.
// checks/risks are always present (possibly empty slices) so the client can
// rely on the shape.
func structuredJSON(source, summary string, checks []checkItem, risks []riskItem) json.RawMessage {
	if checks == nil {
		checks = []checkItem{}
	}
	if risks == nil {
		risks = []riskItem{}
	}
	b, _ := json.Marshal(map[string]any{
		"mock":    true,
		"source":  source,
		"summary": summary,
		"checks":  checks,
		"risks":   risks,
	})
	return b
}

func strPtr(s string) *string { return &s }

var _ ports.Recognizer = (*MockRecognizer)(nil)
