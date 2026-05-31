package auditai

import (
	"encoding/json"
	"testing"
)

func TestInvoiceToResult(t *testing.T) {
	contractor := "ООО Ромашка"
	resp := "Иванов И.И."
	inv := Invoice{
		Number:            "INV-7",
		Date:              "2026-03-15",
		Contractor:        &contractor,
		ResponsiblePerson: &resp,
		Items: []InvoiceItem{
			{Name: "Кофе", Quantity: 3.9, Price: 500.5},
			{Name: "Чай", Quantity: 2, Price: 200},
		},
		GrandTotal: 1701.5,
	}

	res := invoiceToResult(inv, "yc:qwen")

	if res.ModelVersion != "yc:qwen" {
		t.Errorf("model version: %q", res.ModelVersion)
	}
	if res.ExternalNumber == nil || *res.ExternalNumber != "INV-7" {
		t.Errorf("external number: %v", res.ExternalNumber)
	}
	if res.OrganizationName == nil || *res.OrganizationName != contractor {
		t.Errorf("org name: %v", res.OrganizationName)
	}
	if res.DocumentDate == nil || res.DocumentDate.Format("2006-01-02") != "2026-03-15" {
		t.Errorf("document date: %v", res.DocumentDate)
	}
	if len(res.PersonalData) != 1 || res.PersonalData[0] != resp {
		t.Errorf("personal data: %v", res.PersonalData)
	}
	if len(res.ProductNames) != 2 || res.ProductNames[0] != "Кофе" {
		t.Errorf("product names: %v", res.ProductNames)
	}
	if len(res.Quantities) != 2 || res.Quantities[0] != 3 { // int32 truncation
		t.Errorf("quantities: %v", res.Quantities)
	}
	if len(res.Prices) != 2 || res.Prices[0] != "500.5" {
		t.Errorf("prices: %v", res.Prices)
	}
	if len(res.ContractNumbers) != 1 || res.ContractNumbers[0] != "INV-7" {
		t.Errorf("contract numbers: %v", res.ContractNumbers)
	}
	// structured_json must round-trip to the original invoice number.
	var back Invoice
	if err := json.Unmarshal(res.StructuredJSON, &back); err != nil {
		t.Fatalf("structured json invalid: %v", err)
	}
	if back.Number != "INV-7" {
		t.Errorf("structured json number: %q", back.Number)
	}
	if res.RecognizedText == "" {
		t.Error("expected non-empty recognized text")
	}
}

func TestInvoiceToResultEmptyDate(t *testing.T) {
	res := invoiceToResult(Invoice{Number: "X"}, "m")
	if res.DocumentDate != nil {
		t.Errorf("expected nil date for empty/invalid date, got %v", res.DocumentDate)
	}
}
