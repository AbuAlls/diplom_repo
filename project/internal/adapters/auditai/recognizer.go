package auditai

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"diplom.com/m/internal/ports"
)

const dateLayout = "2006-01-02"

// Recognizer adapts the audit AI invoice parser to the ports.Recognizer port,
// so it can be dropped in for the MockRecognizer during document upload.
type Recognizer struct {
	Client *Client
	// Model overrides the client default for recognition when set.
	Model string
}

func (r *Recognizer) Recognize(ctx context.Context, in ports.RecognizeInput) (ports.RecognizeResult, error) {
	inv, err := r.Client.ParseInvoice(ctx, in.FileName, in.MimeType, in.Content, r.Model)
	if err != nil {
		return ports.RecognizeResult{}, err
	}
	return invoiceToResult(inv, r.modelVersion()), nil
}

func (r *Recognizer) modelVersion() string {
	if r.Model != "" {
		return r.Model
	}
	return r.Client.defaultModel
}

// invoiceToResult maps the service Invoice onto the structured document fields
// the rest of the system understands.
func invoiceToResult(inv Invoice, model string) ports.RecognizeResult {
	structured, _ := json.Marshal(inv)

	res := ports.RecognizeResult{
		RecognizedText: invoiceSummary(inv),
		StructuredJSON: structured,
		ModelVersion:   model,
	}

	if inv.Number != "" {
		num := inv.Number
		res.ExternalNumber = &num
		res.ContractNumbers = []string{num}
	}
	if inv.Contractor != nil && *inv.Contractor != "" {
		res.OrganizationName = inv.Contractor
	}
	if t, err := time.Parse(dateLayout, strings.TrimSpace(inv.Date)); err == nil {
		res.DocumentDate = &t
	}

	for _, who := range []*string{inv.ResponsiblePerson, inv.Receiver} {
		if who != nil && strings.TrimSpace(*who) != "" {
			res.PersonalData = append(res.PersonalData, *who)
		}
	}

	for _, it := range inv.Items {
		res.ProductNames = append(res.ProductNames, it.Name)
		res.Quantities = append(res.Quantities, int32(it.Quantity))
		res.Prices = append(res.Prices, strconv.FormatFloat(it.Price, 'f', -1, 64))
	}

	return res
}

func invoiceSummary(inv Invoice) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Invoice %s", inv.Number)
	if inv.Date != "" {
		fmt.Fprintf(&b, " dated %s", inv.Date)
	}
	if inv.Contractor != nil && *inv.Contractor != "" {
		fmt.Fprintf(&b, " from %s", *inv.Contractor)
	}
	fmt.Fprintf(&b, "; %d item(s); total %s", len(inv.Items), strconv.FormatFloat(inv.GrandTotal, 'f', -1, 64))
	return b.String()
}

var (
	_ ports.Recognizer = (*Recognizer)(nil)
	_ ports.Analyzer   = (*Client)(nil)
)
