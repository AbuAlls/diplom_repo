package auditai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseInvoice(t *testing.T) {
	var gotModel, gotImageName, gotContent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/parse-invoice" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotModel = r.URL.Query().Get("model")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		f, hdr, err := r.FormFile("image")
		if err != nil {
			t.Fatalf("form file image: %v", err)
		}
		defer f.Close()
		gotImageName = hdr.Filename
		b, _ := io.ReadAll(f)
		gotContent = string(b)

		contractor := "ООО Ромашка"
		_ = json.NewEncoder(w).Encode(Invoice{
			Number:     "INV-42",
			Date:       "2026-03-15",
			Contractor: &contractor,
			Items: []InvoiceItem{
				{Name: "Кофе", Unit: "kg", Quantity: 3, Price: 500, Total: 1500},
			},
			GrandTotal: 1500,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "default-model", 5*time.Second)
	inv, err := c.ParseInvoice(context.Background(), "scan.png", "image/png", []byte("rawbytes"), "")
	if err != nil {
		t.Fatalf("ParseInvoice: %v", err)
	}
	if gotModel != "default-model" {
		t.Errorf("expected default model passed, got %q", gotModel)
	}
	if gotImageName != "scan.png" {
		t.Errorf("expected image name scan.png, got %q", gotImageName)
	}
	if gotContent != "rawbytes" {
		t.Errorf("expected content rawbytes, got %q", gotContent)
	}
	if inv.Number != "INV-42" || len(inv.Items) != 1 || inv.Items[0].Name != "Кофе" {
		t.Fatalf("unexpected invoice: %+v", inv)
	}
}

func TestParseInvoiceNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "m", time.Second)
	if _, err := c.ParseInvoice(context.Background(), "f.png", "image/png", []byte("x"), ""); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestAnalyze(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent/analyze" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var req analyzeRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "m1" || !strings.Contains(req.Message, "analyze") {
			t.Errorf("unexpected request body: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(analyzeResponse{Recommendations: "do X and Y"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "default", time.Second)
	rec, err := c.Analyze(context.Background(), "m1", "please analyze sales")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if rec != "do X and Y" {
		t.Fatalf("unexpected recommendations: %q", rec)
	}
}
