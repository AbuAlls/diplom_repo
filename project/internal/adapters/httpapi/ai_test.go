package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const testInternalToken = "test-internal-token"

// --- fakes for the AI integration ---

type fakeAnalyzer struct{}

func (fakeAnalyzer) Analyze(_ context.Context, model, message string) (string, error) {
	return "recommendation for: " + message, nil
}

type fakeQueryRepo struct{}

func (fakeQueryRepo) Schema(_ context.Context) (map[string][]string, error) {
	return map[string][]string{
		"documents":  {"id", "title", "status"},
		"plan_items": {"id", "name"},
	}, nil
}

func (fakeQueryRepo) RunReadOnlyQuery(_ context.Context, _ string) ([]string, [][]any, error) {
	return []string{"id", "title"}, [][]any{{int64(1), "Doc A"}, {int64(2), "Doc B"}}, nil
}

func jsonBody(s string) io.Reader { return strings.NewReader(s) }

// --- analyze endpoint (authenticated) ---

func TestAnalyzeItem(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "ai@example.com")
	seedItem(t, srv.URL, token)

	resp := do(t, "POST", srv.URL+"/v0/items/1/analyze", token, `{"message":"анализируй закупки"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("analyze status = %d", resp.StatusCode)
	}
	var out analyzeResponse
	json.NewDecoder(resp.Body).Decode(&out)
	if out.Recommendations != "recommendation for: анализируй закупки" {
		t.Fatalf("unexpected recommendations: %q", out.Recommendations)
	}
}

func TestAnalyzeItemEmptyMessage(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "ai@example.com")
	seedItem(t, srv.URL, token)

	resp := do(t, "POST", srv.URL+"/v0/items/1/analyze", token, `{"message":""}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty message, got %d", resp.StatusCode)
	}
}

func TestAnalyzeItemUnauthorized(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	resp := do(t, "POST", srv.URL+"/v0/items/1/analyze", "", `{"message":"x"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- internal callbacks (shared-secret) ---

func TestGetSchemaWithToken(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	r, _ := http.NewRequest("GET", srv.URL+"/api/schema", nil)
	r.Header.Set("X-Internal-Token", testInternalToken)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("schema req: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("schema status = %d", resp.StatusCode)
	}
	var out map[string][]schemaColumn
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out["documents"]) != 3 || out["documents"][0].Column != "id" {
		t.Fatalf("unexpected schema shape: %+v", out)
	}
}

func TestGetSchemaWrongToken(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	for _, tok := range []string{"", "wrong"} {
		r, _ := http.NewRequest("GET", srv.URL+"/api/schema", nil)
		if tok != "" {
			r.Header.Set("X-Internal-Token", tok)
		}
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatalf("schema req: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("token %q: expected 401, got %d", tok, resp.StatusCode)
		}
	}
}

func TestRunQueryHappy(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	r, _ := http.NewRequest("POST", srv.URL+"/api/analytics/query", jsonBody(`{"query":"select id, title from documents"}`))
	r.Header.Set("X-Internal-Token", testInternalToken)
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("query req: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("query status = %d", resp.StatusCode)
	}
	var out queryResponse
	json.NewDecoder(resp.Body).Decode(&out)
	if out.RowCount != 2 || len(out.Columns) != 2 || out.Columns[0] != "id" {
		t.Fatalf("unexpected query response: %+v", out)
	}
}

func TestRunQueryRejectsNonSelect(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()

	r, _ := http.NewRequest("POST", srv.URL+"/api/analytics/query", jsonBody(`{"query":"delete from documents"}`))
	r.Header.Set("X-Internal-Token", testInternalToken)
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("query req: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on non-SELECT, got %d", resp.StatusCode)
	}
}
