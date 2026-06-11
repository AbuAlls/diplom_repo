package recognition

import (
	"context"
	"encoding/json"
	"testing"

	"diplom.com/m/internal/ports"
)

func recognize(t *testing.T, fileName string) ports.RecognizeResult {
	t.Helper()
	res, err := New().Recognize(context.Background(), ports.RecognizeInput{
		DocumentID: 1,
		FileName:   fileName,
		MimeType:   "application/pdf",
	})
	if err != nil {
		t.Fatalf("Recognize(%q): %v", fileName, err)
	}
	return res
}

type structured struct {
	Mock    bool        `json:"mock"`
	Source  string      `json:"source"`
	Summary string      `json:"summary"`
	Checks  []checkItem `json:"checks"`
	Risks   []riskItem  `json:"risks"`
}

func parseStructured(t *testing.T, raw json.RawMessage) structured {
	t.Helper()
	var s structured
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("structured_json not valid: %v\n%s", err, raw)
	}
	return s
}

func TestExampleContractHasScriptedOutput(t *testing.T) {
	res := recognize(t, "contract_orbita.pdf")

	if res.OrganizationName == nil || *res.OrganizationName != "ООО «Орбита»" {
		t.Fatalf("expected Орбита org, got %v", res.OrganizationName)
	}
	if res.INN == nil || *res.INN != "7704123456" {
		t.Fatalf("expected scripted INN, got %v", res.INN)
	}
	if len(res.Prices) == 0 || res.Prices[0] != "3 200 000 ₽" {
		t.Fatalf("expected scripted price, got %v", res.Prices)
	}

	s := parseStructured(t, res.StructuredJSON)
	if s.Summary == "" {
		t.Fatalf("expected a summary in structured_json")
	}
	if len(s.Checks) != 4 {
		t.Fatalf("expected 4 checks, got %d", len(s.Checks))
	}
	if len(s.Risks) != 2 {
		t.Fatalf("expected 2 risks, got %d", len(s.Risks))
	}
	// One check must be a failed (ok:false) finding — the indexation gap.
	foundFail := false
	for _, c := range s.Checks {
		if !c.OK {
			foundFail = true
		}
	}
	if !foundFail {
		t.Fatalf("expected at least one failed check")
	}
}

func TestExamplesAreDistinct(t *testing.T) {
	contract := recognize(t, "contract_orbita.pdf")
	invoice := recognize(t, "invoice_alfa.pdf")

	if *contract.OrganizationName == *invoice.OrganizationName {
		t.Fatalf("contract and invoice should have different orgs")
	}
	if *contract.INN == *invoice.INN {
		t.Fatalf("contract and invoice should have different INNs")
	}

	cs := parseStructured(t, contract.StructuredJSON)
	is := parseStructured(t, invoice.StructuredJSON)
	if cs.Summary == is.Summary {
		t.Fatalf("summaries should differ between examples")
	}
}

func TestFilenameMatchIsCaseInsensitive(t *testing.T) {
	res := recognize(t, "  Contract_Orbita.PDF ")
	if res.OrganizationName == nil || *res.OrganizationName != "ООО «Орбита»" {
		t.Fatalf("case/space-insensitive match failed, got %v", res.OrganizationName)
	}
}

func TestUnknownFileGetsGenericFallback(t *testing.T) {
	res := recognize(t, "random_upload.pdf")

	if res.OrganizationName == nil || *res.OrganizationName != "ООО Ромашка" {
		t.Fatalf("expected generic org, got %v", res.OrganizationName)
	}
	s := parseStructured(t, res.StructuredJSON)
	if s.Summary == "" {
		t.Fatalf("generic result should still have a summary")
	}
	if len(s.Checks) != 0 || len(s.Risks) != 0 {
		t.Fatalf("generic result should have empty checks/risks, got %d/%d", len(s.Checks), len(s.Risks))
	}
	// Shape must be consistent: checks/risks present as arrays, not null.
	if s.Checks == nil || s.Risks == nil {
		t.Fatalf("checks/risks must be empty arrays, not null")
	}
}

func TestAllCatalogEntriesRecognize(t *testing.T) {
	for name := range exampleCatalog {
		res := recognize(t, name)
		s := parseStructured(t, res.StructuredJSON)
		if s.Source != name {
			t.Errorf("%s: structured.source = %q", name, s.Source)
		}
		if res.RecognizedText == "" {
			t.Errorf("%s: empty recognized text", name)
		}
	}
}
