// Package auditai is a thin HTTP client ("mini-client") for the external
// business-audit-analytics service (FastAPI). It exposes invoice parsing and the
// analytics agent, and adapts invoice parsing to the ports.Recognizer port.
package auditai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Client talks to the business-audit-analytics service.
type Client struct {
	baseURL      string
	defaultModel string
	http         *http.Client
}

// NewClient builds a client. baseURL is the service root (e.g. http://localhost:8000),
// model is the default LLM identifier (e.g. yc:qwen3.5-35b-a3b).
func NewClient(baseURL, model string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		baseURL:      trimSlash(baseURL),
		defaultModel: model,
		http:         &http.Client{Timeout: timeout},
	}
}

// InvoiceItem mirrors the service's InvoiceItem schema.
type InvoiceItem struct {
	Name     string  `json:"name"`
	Unit     string  `json:"unit"`
	Quantity float64 `json:"quantity"`
	Price    float64 `json:"price"`
	Total    float64 `json:"total"`
	Category *string `json:"category"`
}

// Invoice mirrors the service's Invoice schema returned by /parse-invoice.
type Invoice struct {
	Number            string        `json:"number"`
	Date              string        `json:"date"`
	Contractor        *string       `json:"contractor"`
	ResponsiblePerson *string       `json:"responsible_person"`
	Receiver          *string       `json:"receiver"`
	Items             []InvoiceItem `json:"items"`
	GrandTotal        float64       `json:"grand_total"`
	PaymentMethod     *string       `json:"payment_method"`
}

type analyzeRequest struct {
	Model   string `json:"model"`
	Message string `json:"message"`
}

type analyzeResponse struct {
	Recommendations string `json:"recommendations"`
}

type modelsResponse struct {
	Models []string `json:"models"`
}

// ParseInvoice posts the file as multipart/form-data (field "image") to
// /parse-invoice and decodes the structured Invoice. An empty model falls back
// to the client default.
func (c *Client) ParseInvoice(ctx context.Context, fileName, mimeType string, content []byte, model string) (Invoice, error) {
	if model == "" {
		model = c.defaultModel
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("image", fileName)
	if err != nil {
		return Invoice{}, err
	}
	if _, err := part.Write(content); err != nil {
		return Invoice{}, err
	}
	if err := mw.Close(); err != nil {
		return Invoice{}, err
	}

	endpoint := c.baseURL + "/parse-invoice"
	if model != "" {
		endpoint += "?model=" + url.QueryEscape(model)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return Invoice{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	var inv Invoice
	if err := c.do(req, &inv); err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

// Analyze runs the analytics agent and returns its recommendations text.
func (c *Client) Analyze(ctx context.Context, model, message string) (string, error) {
	if model == "" {
		model = c.defaultModel
	}
	payload, err := json.Marshal(analyzeRequest{Model: model, Message: message})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/agent/analyze", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	var resp analyzeResponse
	if err := c.do(req, &resp); err != nil {
		return "", err
	}
	return resp.Recommendations, nil
}

// ListModels returns the model identifiers the service advertises.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	var resp modelsResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Models, nil
}

// do executes the request and decodes a 2xx JSON body into out, or returns an
// error carrying the status and a truncated body on failure.
func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("audit ai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("audit ai %s %s: status %d: %s", req.Method, req.URL.Path, resp.StatusCode, bytes.TrimSpace(snippet))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("audit ai decode %s: %w", req.URL.Path, err)
	}
	return nil
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
