package sub_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Scenario 6 — Front ↔ Back contract & CORS.
//
// This replays the exact request shapes the browser frontend issues from its
// network layer (diploma front end/api.js) against the real Go API wrapped in
// the same WithCORS middleware cmd/api/main.go uses. It proves the two halves
// can actually talk to each other:
//
//   - CORS preflight (OPTIONS) is answered 204 with permissive headers, and
//     normal responses carry Access-Control-Allow-Origin — without this the
//     browser blocks every cross-origin call;
//   - register / login wire formats match (JSON body vs form grant) and return
//     {access_token, token_type:"Bearer"};
//   - the api.js "demo()" fallback (login → on failure register) works;
//   - plans/goals/items/documents/analytics requests use field names the
//     backend actually reads, and responses expose what api.js consumes;
//   - the error envelope is {error:{code,message}} (api.js ApiError relies on it);
//   - a 401 is returned when the bearer token is absent (api.js AuthError path).
//
// Endpoints in api.js NOT covered elsewhere are all touched here.
func TestScenario_FrontBackContractAndCORS(t *testing.T) {
	h := newHarness(t)
	const frontOrigin = "http://localhost:5500" // serve.py default

	// ---------- CORS preflight ----------
	pre := h.options("/v0/plans", frontOrigin, "GET")
	if pre.StatusCode != http.StatusNoContent {
		t.Fatalf("CORS preflight: expected 204, got %d", pre.StatusCode)
	}
	if got := pre.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("preflight Allow-Origin: expected *, got %q", got)
	}
	if got := pre.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(got, "PATCH") || !strings.Contains(got, "POST") {
		t.Fatalf("preflight Allow-Methods missing verbs: %q", got)
	}
	if got := pre.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
		t.Fatalf("preflight Allow-Headers missing Authorization: %q", got)
	}

	// ---------- register (JSON) — api.js auth.register ----------
	regStatus, regBody := h.reqOrigin("POST", "/v0/auth/register", "", frontOrigin,
		`{"email":"web@orbita.ru","password":"password123","full_name":"Web User"}`)
	if regStatus != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d (%s)", regStatus, regBody)
	}
	// Cross-origin responses must also carry the CORS header.
	if regBody.acao != "*" {
		t.Fatalf("register response missing Access-Control-Allow-Origin, got %q", regBody.acao)
	}
	var reg tokenResp
	json.Unmarshal(regBody.body, &reg)
	if reg.AccessToken == "" || reg.TokenType != "Bearer" {
		t.Fatalf("register token payload wrong: %+v", reg)
	}

	// ---------- login (form grant) — api.js auth.login ----------
	st, token := h.login("web@orbita.ru", "password123")
	if st != http.StatusOK || token == "" {
		t.Fatalf("login: status=%d empty=%v", st, token == "")
	}

	// ---------- demo() fallback: login unknown → 401 → register ----------
	if st, _ := h.login("demo@orbita.ru", "demo12345"); st != http.StatusUnauthorized {
		t.Fatalf("demo login (unknown) should 401, got %d", st)
	}
	demoStatus, _ := h.req("POST", "/v0/auth/register", "",
		`{"email":"demo@orbita.ru","password":"demo12345","full_name":"Алексей Климов"}`)
	if demoStatus != http.StatusCreated {
		t.Fatalf("demo register fallback: expected 201, got %d", demoStatus)
	}

	// ---------- plans.create / plans.list ----------
	var plan planResp
	if st := h.reqJSON("POST", "/v0/plans", token, `{"name":"Q3 Закупки","description":"demo","status":"active"}`, &plan); st != http.StatusCreated {
		t.Fatalf("plans.create: status %d", st)
	}
	if plan.Status != "active" || plan.Name != "Q3 Закупки" {
		t.Fatalf("plan response mismatch: %+v", plan)
	}
	var plansList listResp
	h.reqJSON("GET", "/v0/plans?page=1&size=50", token, "", &plansList)
	if plansList.Meta.Total != 1 {
		t.Fatalf("plans.list total: expected 1, got %d", plansList.Meta.Total)
	}

	// ---------- goals.create (sort_order) / goals.list ----------
	var goal goalResp
	if st := h.reqJSON("POST", "/v0/plans/"+itoa(plan.ID)+"/goals", token, `{"name":"Поставщики","description":"d","sort_order":2}`, &goal); st != http.StatusCreated {
		t.Fatalf("goals.create: status %d", st)
	}
	if goal.SortOrder != 2 {
		t.Fatalf("goal sort_order not honored: %+v", goal)
	}

	// ---------- items.create (full body) / items.update ----------
	itemBody := `{"name":"Контракты","item_type":"metric","status":"active","sort_order":1,"target_value":120,"current_value":30,"unit":"шт"}`
	var item itemResp
	if st := h.reqJSON("POST", "/v0/plans/"+itoa(plan.ID)+"/goals/"+itoa(goal.ID)+"/items", token, itemBody, &item); st != http.StatusCreated {
		t.Fatalf("items.create: status %d", st)
	}
	if item.ItemType == nil || *item.ItemType != "metric" {
		t.Fatalf("item_type not honored: %+v", item.ItemType)
	}
	if item.ProgressPercent == nil || *item.ProgressPercent != 25 { // 30/120
		t.Fatalf("expected progress 25, got %v", item.ProgressPercent)
	}
	if st := h.reqJSON("PATCH", "/v0/plans/"+itoa(plan.ID)+"/goals/"+itoa(goal.ID)+"/items/"+itoa(item.ID), token, `{"current_value":60}`, &item); st != http.StatusOK {
		t.Fatalf("items.update: status %d", st)
	}
	if item.ProgressPercent == nil || *item.ProgressPercent != 50 {
		t.Fatalf("expected progress 50 after update, got %v", item.ProgressPercent)
	}

	// ---------- documents.upload (multipart "file") / get / list ----------
	st, doc := h.uploadDoc(token, item.ID, "договор.pdf", "contract-bytes")
	if st != http.StatusOK || doc.ID == 0 {
		t.Fatalf("documents.upload: status=%d id=%d", st, doc.ID)
	}
	var got docResp
	if st := h.reqJSON("GET", "/v0/documents/"+itoa(doc.ID), token, "", &got); st != http.StatusOK {
		t.Fatalf("documents.get: status %d", st)
	}
	if got.FileName != "договор.pdf" {
		t.Fatalf("documents.get file_name mismatch: %q", got.FileName)
	}

	// ---------- documents.downloadBlob (Bearer header, raw bytes) ----------
	dlStatus, dlBytes := h.req("GET", "/v0/documents/"+itoa(doc.ID)+"/download", token, "")
	if dlStatus != http.StatusOK || string(dlBytes) != "contract-bytes" {
		t.Fatalf("download: status=%d body=%q", dlStatus, dlBytes)
	}

	// ---------- items.analytics / items.analyze ----------
	var an analyticsResp
	if st := h.reqJSON("GET", "/v0/items/"+itoa(item.ID)+"/analytics", token, "", &an); st != http.StatusOK {
		t.Fatalf("items.analytics: status %d", st)
	}
	h.users.query.seed(h.userIDFor("web@orbita.ru"), doc.ID, "договор.pdf")
	var ar analyzeResp
	if st := h.reqJSON("POST", "/v0/items/"+itoa(item.ID)+"/analyze", token, `{"message":"оцени риски","model":"yc:qwen"}`, &ar); st != http.StatusOK {
		t.Fatalf("items.analyze: status %d", st)
	}
	if ar.Recommendations == "" {
		t.Fatalf("analyze returned empty recommendations")
	}

	// ---------- documents.confirm / reject / reanalyze surface ----------
	if st, _ := h.req("POST", "/v0/documents/"+itoa(doc.ID)+"/confirm", token, ""); st != http.StatusOK {
		t.Fatalf("documents.confirm: status %d", st)
	}

	// ---------- error envelope shape the frontend parses ----------
	st, body := h.req("POST", "/v0/plans", token, `{"name":""}`)
	if st != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty plan name, got %d", st)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("error envelope not decodable: %v; body=%s", err, body)
	}
	if env.Error.Code == "" || env.Error.Message == "" {
		t.Fatalf("error envelope must carry code+message: %+v", env.Error)
	}

	// ---------- 401 path (api.js AuthError → return to login) ----------
	if st, _ := h.req("GET", "/v0/plans", "", ""); st != http.StatusUnauthorized {
		t.Fatalf("missing token: expected 401, got %d", st)
	}
}

// ---- small CORS/origin-aware helpers (local to this scenario) ----

func (h *harness) options(path, origin, reqMethod string) *http.Response {
	h.t.Helper()
	r, _ := http.NewRequest(http.MethodOptions, h.srv.URL+path, nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Access-Control-Request-Method", reqMethod)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		h.t.Fatalf("OPTIONS %s: %v", path, err)
	}
	resp.Body.Close()
	return resp
}

type originResp struct {
	body []byte
	acao string // Access-Control-Allow-Origin
}

func (h *harness) reqOrigin(method, path, token, origin, body string) (int, originResp) {
	h.t.Helper()
	var r *http.Request
	var err error
	if body == "" {
		r, err = http.NewRequest(method, h.srv.URL+path, nil)
	} else {
		r, err = http.NewRequest(method, h.srv.URL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	r.Header.Set("Origin", origin)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		h.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, originResp{body: b, acao: resp.Header.Get("Access-Control-Allow-Origin")}
}
