// Package sub_test holds black-box-style integration tests that drive the full
// HTTP API over a real httptest.Server. Unlike the in-package handler tests
// under internal/adapters/httpapi, this suite lives in its own directory and
// wires the API exactly the way cmd/api/main.go does (notably: a SINGLE shared
// SessionStore), so the complete AI-agent callback loop — analyze → session
// nonce → /api/schema + /api/analytics/query — can be exercised end to end.
//
// Everything is in-memory (no Postgres / MinIO required); run with:
//
//	go test ./internal/sub_test/...
package sub_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"diplom.com/m/internal/adapters/recognition"
	"diplom.com/m/internal/auth"
	httpapi "diplom.com/m/internal/adapters/httpapi"
	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"diplom.com/m/internal/usecase"
)

const internalToken = "sub-test-internal-token"

// =====================================================================
// In-memory adapters (implement the ports.* interfaces).
//
// These mirror the fakes used by the httpapi package tests, but are
// re-declared here because those are unexported test-only symbols that
// cannot be imported across package boundaries.
// =====================================================================

type memUserRepo struct {
	byID    map[int64]ports.UserDTO
	byEmail map[string]ports.UserDTO
	nextID  int64
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{byID: map[int64]ports.UserDTO{}, byEmail: map[string]ports.UserDTO{}}
}

func (m *memUserRepo) Create(_ context.Context, email, fullName, hash string) (int64, error) {
	m.nextID++
	u := ports.UserDTO{ID: m.nextID, Email: email, FullName: fullName, PasswordHash: hash}
	m.byID[u.ID] = u
	m.byEmail[email] = u
	return u.ID, nil
}

func (m *memUserRepo) GetByEmail(_ context.Context, email string) (ports.UserDTO, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return ports.UserDTO{}, ports.ErrNotFound
	}
	return u, nil
}

func (m *memUserRepo) GetByID(_ context.Context, id int64) (ports.UserDTO, error) {
	u, ok := m.byID[id]
	if !ok {
		return ports.UserDTO{}, ports.ErrNotFound
	}
	return u, nil
}

type memPlanRepo struct {
	byID   map[int64]domain.Plan
	order  []int64
	nextID int64
}

func newMemPlanRepo() *memPlanRepo { return &memPlanRepo{byID: map[int64]domain.Plan{}} }

func (m *memPlanRepo) Create(_ context.Context, ownerID int64, name string, desc *string, status string) (domain.Plan, error) {
	m.nextID++
	p := domain.Plan{ID: m.nextID, Name: name, Description: desc, CreatedBy: ownerID, Status: status, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.byID[p.ID] = p
	m.order = append(m.order, p.ID)
	return p, nil
}

func (m *memPlanRepo) GetByID(_ context.Context, id int64) (domain.Plan, error) {
	p, ok := m.byID[id]
	if !ok {
		return domain.Plan{}, ports.ErrNotFound
	}
	return p, nil
}

func (m *memPlanRepo) ListByOwner(_ context.Context, ownerID int64, offset, limit int) ([]domain.Plan, int, error) {
	// Iterate in insertion order so pagination is deterministic.
	var all []domain.Plan
	for _, id := range m.order {
		if p := m.byID[id]; p.CreatedBy == ownerID {
			all = append(all, p)
		}
	}
	total := len(all)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

type memGoalRepo struct {
	byID   map[int64]domain.Goal
	nextID int64
}

func newMemGoalRepo() *memGoalRepo { return &memGoalRepo{byID: map[int64]domain.Goal{}} }

func (m *memGoalRepo) Create(_ context.Context, planID int64, name string, desc *string, sortOrder int) (domain.Goal, error) {
	m.nextID++
	g := domain.Goal{ID: m.nextID, PlanID: planID, Name: name, Description: desc, SortOrder: sortOrder}
	m.byID[g.ID] = g
	return g, nil
}

func (m *memGoalRepo) GetByID(_ context.Context, id int64) (domain.Goal, error) {
	g, ok := m.byID[id]
	if !ok {
		return domain.Goal{}, ports.ErrNotFound
	}
	return g, nil
}

func (m *memGoalRepo) ListByPlan(_ context.Context, planID int64, offset, limit int) ([]domain.Goal, int, error) {
	var all []domain.Goal
	for _, g := range m.byID {
		if g.PlanID == planID {
			all = append(all, g)
		}
	}
	return all, len(all), nil
}

type memItemRepo struct {
	byID   map[int64]domain.PlanItem
	nextID int64
}

func newMemItemRepo() *memItemRepo { return &memItemRepo{byID: map[int64]domain.PlanItem{}} }

func (m *memItemRepo) Create(_ context.Context, goalID int64, in ports.PlanItemInput) (domain.PlanItem, error) {
	m.nextID++
	it := domain.PlanItem{
		ID: m.nextID, GoalID: goalID, Name: in.Name, Description: in.Description,
		ItemType: &in.ItemType, Status: &in.Status, TargetValue: in.TargetValue,
		CurrentValue: in.CurrentValue, Unit: in.Unit, SortOrder: in.SortOrder,
	}
	m.byID[it.ID] = it
	return it, nil
}

func (m *memItemRepo) GetByID(_ context.Context, id int64) (domain.PlanItem, error) {
	it, ok := m.byID[id]
	if !ok {
		return domain.PlanItem{}, ports.ErrNotFound
	}
	return it, nil
}

func (m *memItemRepo) ListByGoal(_ context.Context, goalID int64, offset, limit int) ([]domain.PlanItem, int, error) {
	var all []domain.PlanItem
	for _, it := range m.byID {
		if it.GoalID == goalID {
			all = append(all, it)
		}
	}
	return all, len(all), nil
}

func (m *memItemRepo) Update(_ context.Context, id int64, patch ports.PlanItemPatch) (domain.PlanItem, error) {
	it, ok := m.byID[id]
	if !ok {
		return domain.PlanItem{}, ports.ErrNotFound
	}
	if patch.Name != nil {
		it.Name = *patch.Name
	}
	if patch.CurrentValue != nil {
		it.CurrentValue = patch.CurrentValue
	}
	if patch.TargetValue != nil {
		it.TargetValue = patch.TargetValue
	}
	if patch.Status != nil {
		it.Status = patch.Status
	}
	m.byID[id] = it
	return it, nil
}

type memFolderRepo struct {
	byItem map[int64]int64
	nextID int64
}

func newMemFolderRepo() *memFolderRepo { return &memFolderRepo{byItem: map[int64]int64{}} }

func (m *memFolderRepo) FindOrCreateItemFolder(_ context.Context, planItemID, _ int64) (int64, error) {
	if id, ok := m.byItem[planItemID]; ok {
		return id, nil
	}
	m.nextID++
	m.byItem[planItemID] = m.nextID
	return m.nextID, nil
}

type memDocumentRepo struct {
	byID   map[int64]domain.Document
	nextID int64
}

func newMemDocumentRepo() *memDocumentRepo { return &memDocumentRepo{byID: map[int64]domain.Document{}} }

func (m *memDocumentRepo) Create(_ context.Context, in ports.DocumentCreate) (domain.Document, error) {
	m.nextID++
	now := time.Now()
	d := domain.Document{
		ID: m.nextID, PlanItemID: in.PlanItemID, Title: in.Title, FolderID: in.FolderID,
		UploadedBy: in.UploadedBy, Status: in.Status, FileName: in.FileName, FilePath: in.FilePath,
		MimeType: in.MimeType, FileSize: in.FileSize, CreatedAt: now, UpdatedAt: now,
	}
	m.byID[d.ID] = d
	return d, nil
}

func (m *memDocumentRepo) GetByID(_ context.Context, id int64) (domain.Document, error) {
	d, ok := m.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	return d, nil
}

func (m *memDocumentRepo) ListByOwner(_ context.Context, ownerID int64, planItemID *int64, offset, limit int) ([]domain.Document, int, error) {
	var all []domain.Document
	for i := int64(1); i <= m.nextID; i++ { // stable id order
		d, ok := m.byID[i]
		if !ok || d.UploadedBy != ownerID {
			continue
		}
		if planItemID != nil && d.PlanItemID != *planItemID {
			continue
		}
		all = append(all, d)
	}
	total := len(all)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

func (m *memDocumentRepo) UpdateFields(_ context.Context, id int64, patch ports.DocumentPatch) (domain.Document, error) {
	d, ok := m.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	if patch.DocumentDate != nil {
		d.DocumentDate = patch.DocumentDate
	}
	if patch.ExternalNumber != nil {
		d.ExternalNumber = patch.ExternalNumber
	}
	if patch.OrganizationName != nil {
		d.OrganizationName = patch.OrganizationName
	}
	if patch.INN != nil {
		d.INN = patch.INN
	}
	if patch.Description != nil {
		d.Description = patch.Description
	}
	if patch.Deadlines != nil {
		d.Deadlines = patch.Deadlines
	}
	if patch.PersonalData != nil {
		d.PersonalData = patch.PersonalData
	}
	if patch.OrganizationData != nil {
		d.OrganizationData = patch.OrganizationData
	}
	if patch.Prices != nil {
		d.Prices = patch.Prices
	}
	if patch.Quantities != nil {
		d.Quantities = patch.Quantities
	}
	if patch.ProductNames != nil {
		d.ProductNames = patch.ProductNames
	}
	if patch.ContractNumbers != nil {
		d.ContractNumbers = patch.ContractNumbers
	}
	d.UpdatedAt = time.Now()
	m.byID[id] = d
	return d, nil
}

func (m *memDocumentRepo) UpdateStatus(_ context.Context, id int64, status string) (domain.Document, error) {
	d, ok := m.byID[id]
	if !ok {
		return domain.Document{}, ports.ErrNotFound
	}
	d.Status = status
	d.UpdatedAt = time.Now()
	m.byID[id] = d
	return d, nil
}

func (m *memDocumentRepo) CountByPlanItem(_ context.Context, planItemID int64) (int, error) {
	n := 0
	for _, d := range m.byID {
		if d.PlanItemID == planItemID {
			n++
		}
	}
	return n, nil
}

func (m *memDocumentRepo) LatestDocIDByPlanItem(_ context.Context, planItemID int64) (*int64, error) {
	var latest int64
	for _, d := range m.byID {
		if d.PlanItemID == planItemID && d.ID > latest {
			latest = d.ID
		}
	}
	if latest == 0 {
		return nil, nil
	}
	return &latest, nil
}

type memExtractedRepo struct {
	byDoc map[int64]domain.ExtractedData
}

func newMemExtractedRepo() *memExtractedRepo {
	return &memExtractedRepo{byDoc: map[int64]domain.ExtractedData{}}
}

func (m *memExtractedRepo) Create(_ context.Context, in ports.ExtractedDataCreate) (domain.ExtractedData, error) {
	e := domain.ExtractedData{
		ID: in.DocumentID, DocumentID: in.DocumentID, RecognizedText: in.RecognizedText,
		StructuredJSON: in.StructuredJSON, RecognizedCategoryID: in.RecognizedCategoryID,
		ConfidenceScore: in.ConfidenceScore, ProcessingStatus: in.ProcessingStatus,
		ProcessedAt: in.ProcessedAt, ModelVersion: in.ModelVersion,
	}
	m.byDoc[in.DocumentID] = e
	return e, nil
}

func (m *memExtractedRepo) GetByDocumentID(_ context.Context, documentID int64) (domain.ExtractedData, error) {
	e, ok := m.byDoc[documentID]
	if !ok {
		return domain.ExtractedData{}, ports.ErrNotFound
	}
	return e, nil
}

func (m *memExtractedRepo) ListByDocumentIDs(_ context.Context, ids []int64) (map[int64]domain.ExtractedData, error) {
	out := make(map[int64]domain.ExtractedData, len(ids))
	for _, id := range ids {
		if e, ok := m.byDoc[id]; ok {
			out[id] = e
		}
	}
	return out, nil
}

func (m *memExtractedRepo) UpdateCategory(_ context.Context, documentID int64, categoryID *int64) error {
	e, ok := m.byDoc[documentID]
	if !ok {
		return ports.ErrNotFound
	}
	e.RecognizedCategoryID = categoryID
	m.byDoc[documentID] = e
	return nil
}

type memFileStore struct {
	saved map[string][]byte
}

func newMemFileStore() *memFileStore { return &memFileStore{saved: map[string][]byte{}} }

func (m *memFileStore) Save(_ context.Context, key string, r io.Reader) (int64, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	m.saved[key] = b
	return int64(len(b)), nil
}

func (m *memFileStore) Stat(_ context.Context, key string) (ports.FileStatus, error) {
	b, ok := m.saved[key]
	if !ok {
		return ports.FileStatus{Key: key, Exists: false, StatusCode: http.StatusNotFound}, nil
	}
	n := int64(len(b))
	return ports.FileStatus{Key: key, Exists: true, StatusCode: http.StatusOK, ContentLength: &n, ContentType: "application/octet-stream", ETag: `"etag"`}, nil
}

func (m *memFileStore) Open(_ context.Context, key string) (ports.FileObject, error) {
	b, ok := m.saved[key]
	if !ok {
		return ports.FileObject{}, ports.ErrNotFound
	}
	n := int64(len(b))
	return ports.FileObject{
		Status: ports.FileStatus{Key: key, Exists: true, StatusCode: http.StatusOK, ContentLength: &n, ContentType: "text/plain", ETag: `"etag"`},
		Body:   io.NopCloser(bytes.NewReader(b)),
	}, nil
}

// =====================================================================
// AI fakes: a scripted analyzer that actually drives the agent callback
// loop, plus an owner-scoped query repo that honours per-tenant ownerID.
// =====================================================================

// scopedQueryRepo simulates the production AnalyticsQueryRepo: rows are tagged
// with an owner, and a query scoped to ownerID > 0 only ever returns that
// owner's rows. ownerID == 0 (global token) sees everything — matching the
// "unscoped / ops only" semantics documented in ports.go.
type scopedQueryRepo struct {
	// rowsByOwner[ownerID] = list of {id, title} document rows.
	rowsByOwner map[int64][][2]any
}

func newScopedQueryRepo() *scopedQueryRepo {
	return &scopedQueryRepo{rowsByOwner: map[int64][][2]any{}}
}

func (q *scopedQueryRepo) seed(ownerID int64, id int64, title string) {
	q.rowsByOwner[ownerID] = append(q.rowsByOwner[ownerID], [2]any{id, title})
}

func (q *scopedQueryRepo) Schema(_ context.Context) (map[string][]string, error) {
	return map[string][]string{
		"plans":      {"id", "name", "created_by", "status"},
		"plan_items": {"id", "goal_id", "name", "current_value", "target_value"},
		"documents":  {"id", "plan_item_id", "title", "status", "owner_id"},
	}, nil
}

func (q *scopedQueryRepo) RunReadOnlyQuery(_ context.Context, _ string, ownerID int64) ([]string, [][]any, error) {
	cols := []string{"id", "title", "owner_id"}
	var rows [][]any
	emit := func(owner int64) {
		for _, r := range q.rowsByOwner[owner] {
			rows = append(rows, []any{r[0], r[1], owner})
		}
	}
	if ownerID > 0 {
		emit(ownerID) // per-tenant scoping
	} else {
		for owner := range q.rowsByOwner { // global token: everything
			emit(owner)
		}
	}
	return cols, rows, nil
}

var nonceRE = regexp.MustCompile(`X-Internal-Token: ([0-9a-f]+)`)

// scriptedAnalyzer plays the role of the external AI service. When Analyze is
// invoked it parses the injected session nonce out of the message and performs
// the same callbacks a real agent would: GET /api/schema then POST
// /api/analytics/query, both authenticated with that nonce. It records what it
// observed so scenarios can assert on the full round trip.
type scriptedAnalyzer struct {
	baseURL string // set by the harness after the server starts

	lastNonce   string
	calledModel string
	schemaSeen  map[string][]schemaColumn
	rowsSeen    [][]any
	httpErr     error
}

func (a *scriptedAnalyzer) Analyze(_ context.Context, model, message string) (string, error) {
	a.calledModel = model

	mm := nonceRE.FindStringSubmatch(message)
	if mm == nil {
		a.httpErr = errNoNonce
		return "", errNoNonce
	}
	a.lastNonce = mm[1]

	// 1) Schema introspection.
	var schema map[string][]schemaColumn
	if st, err := a.internalGET("/api/schema", a.lastNonce, &schema); err != nil || st != http.StatusOK {
		a.httpErr = errCallback
		return "", errCallback
	}
	a.schemaSeen = schema

	// 2) Read-only query, scoped to the initiating user by the nonce.
	var qr queryResp
	if st, err := a.internalPOST("/api/analytics/query", a.lastNonce, `{"query":"select id, title, owner_id from documents"}`, &qr); err != nil || st != http.StatusOK {
		a.httpErr = errCallback
		return "", errCallback
	}
	a.rowsSeen = qr.Rows

	return "analyzed " + strconv.Itoa(qr.RowCount) + " documents", nil
}

func (a *scriptedAnalyzer) internalGET(path, token string, out any) (int, error) {
	r, _ := http.NewRequest("GET", a.baseURL+path, nil)
	r.Header.Set("X-Internal-Token", token)
	return doDecode(r, out)
}

func (a *scriptedAnalyzer) internalPOST(path, token, body string, out any) (int, error) {
	r, _ := http.NewRequest("POST", a.baseURL+path, strings.NewReader(body))
	r.Header.Set("X-Internal-Token", token)
	r.Header.Set("Content-Type", "application/json")
	return doDecode(r, out)
}

func doDecode(r *http.Request, out any) (int, error) {
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if out != nil && resp.StatusCode == http.StatusOK {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, nil
}

var (
	errNoNonce  = errStr("analyzer: no session nonce found in message")
	errCallback = errStr("analyzer: internal callback failed")
)

type errStr string

func (e errStr) Error() string { return string(e) }

// =====================================================================
// Harness + HTTP client helpers.
// =====================================================================

type harness struct {
	t     *testing.T
	srv   *httptest.Server
	users *memPlanReposBundle
}

// memPlanReposBundle keeps references to the fakes scenarios want to inspect.
type memPlanReposBundle struct {
	users    *memUserRepo
	query    *scopedQueryRepo
	analyzer *scriptedAnalyzer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	users := newMemUserRepo()
	plans := newMemPlanRepo()
	goals := newMemGoalRepo()
	items := newMemItemRepo()
	folders := newMemFolderRepo()
	docs := newMemDocumentRepo()
	extracted := newMemExtractedRepo()
	store := newMemFileStore()
	query := newScopedQueryRepo()
	analyzer := &scriptedAnalyzer{}

	// CRITICAL: a single shared SessionStore, exactly like cmd/api/main.go.
	// This is what lets a nonce minted inside Recommendations be resolved by
	// the internalAuth middleware on the /api/* callbacks.
	sessions := usecase.NewSessionStore()

	tm := auth.TokenManager{Secret: []byte("sub-test-secret"), Issuer: "sub-test"}
	api := &httpapi.API{
		Auth:  &usecase.AuthService{Users: users, Tokens: tm, AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour},
		Plans: &usecase.PlanService{Plans: plans},
		Goals: &usecase.GoalService{Goals: goals, Plans: plans},
		Items: &usecase.PlanItemService{Items: items, Goals: goals, Plans: plans},
		Docs: &usecase.DocumentService{
			Docs: docs, Folders: folders, Items: items, Goals: goals, Plans: plans,
			Store: store, Extracted: extracted, Recognizer: recognition.New(),
		},
		Analytics: &usecase.AnalyticsService{
			Items: items, Goals: goals, Plans: plans, Docs: docs,
			Analyzer: analyzer, Sessions: sessions,
		},
		InternalAnalytics: &usecase.InternalAnalyticsService{Query: query},
		InternalToken:     internalToken,
		Sessions:          sessions,
	}

	// Wrap with WithCORS exactly like cmd/api/main.go, so the browser-facing
	// CORS behavior the frontend depends on is exercised too.
	srv := httptest.NewServer(httpapi.WithCORS(api.Routes(), "*"))
	analyzer.baseURL = srv.URL // close the construction loop
	t.Cleanup(srv.Close)

	return &harness{t: t, srv: srv, users: &memPlanReposBundle{users: users, query: query, analyzer: analyzer}}
}

// userIDFor returns the numeric id the user repo assigned to an email.
func (h *harness) userIDFor(email string) int64 {
	return h.users.users.byEmail[email].ID
}

// register creates a user and returns its bearer access token.
func (h *harness) register(email string) string {
	h.t.Helper()
	body := `{"email":"` + email + `","password":"password123","full_name":"Sub Test"}`
	resp, err := http.Post(h.srv.URL+"/v0/auth/register", "application/json", strings.NewReader(body))
	if err != nil {
		h.t.Fatalf("register %s: %v", email, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		h.t.Fatalf("register %s: status %d", email, resp.StatusCode)
	}
	var tr tokenResp
	json.NewDecoder(resp.Body).Decode(&tr)
	if tr.AccessToken == "" {
		h.t.Fatalf("register %s: empty access token", email)
	}
	return tr.AccessToken
}

// login exercises the password-grant token endpoint (form-encoded).
func (h *harness) login(email, password string) (int, string) {
	h.t.Helper()
	form := url.Values{"grant_type": {"password"}, "username": {email}, "password": {password}}
	resp, err := http.PostForm(h.srv.URL+"/v0/auth/token", form)
	if err != nil {
		h.t.Fatalf("login %s: %v", email, err)
	}
	defer resp.Body.Close()
	var tr tokenResp
	json.NewDecoder(resp.Body).Decode(&tr)
	return resp.StatusCode, tr.AccessToken
}

// req performs a JSON request and returns status + raw body.
func (h *harness) req(method, path, token, body string) (int, []byte) {
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
		h.t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		h.t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// reqJSON is req + JSON-decode of the body into out (when status is 2xx).
func (h *harness) reqJSON(method, path, token, body string, out any) int {
	h.t.Helper()
	st, b := h.req(method, path, token, body)
	if out != nil && st >= 200 && st < 300 {
		if err := json.Unmarshal(b, out); err != nil {
			h.t.Fatalf("decode %s %s (status %d): %v; body=%s", method, path, st, err, b)
		}
	}
	return st
}

// internalReq hits an /api/* callback with the given X-Internal-Token.
func (h *harness) internalReq(method, path, token, body string) (int, []byte) {
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
		h.t.Fatalf("new internal request: %v", err)
	}
	if token != "" {
		r.Header.Set("X-Internal-Token", token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		h.t.Fatalf("do internal %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// uploadDoc posts a multipart document to an owned plan item.
func (h *harness) uploadDoc(token string, itemID int64, fileName, content string) (int, docResp) {
	h.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", fileName)
	if err != nil {
		h.t.Fatalf("create form file: %v", err)
	}
	fw.Write([]byte(content))
	mw.Close()

	r, _ := http.NewRequest("POST", h.srv.URL+"/v0/documents/upload/"+strconv.FormatInt(itemID, 10), &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		h.t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	var d docResp
	if resp.StatusCode == http.StatusOK {
		json.NewDecoder(resp.Body).Decode(&d)
	}
	return resp.StatusCode, d
}

// seedItem builds a plan→goal→item tree owned by token and returns their ids.
func (h *harness) seedItem(token, planName string, target, current float64) (planID, goalID, itemID int64) {
	h.t.Helper()
	var p planResp
	if st := h.reqJSON("POST", "/v0/plans", token, `{"name":"`+planName+`"}`, &p); st != http.StatusCreated {
		h.t.Fatalf("seed plan: status %d", st)
	}
	var g goalResp
	if st := h.reqJSON("POST", "/v0/plans/"+itoa(p.ID)+"/goals", token, `{"name":"Goal"}`, &g); st != http.StatusCreated {
		h.t.Fatalf("seed goal: status %d", st)
	}
	var it itemResp
	body := `{"name":"Item","target_value":` + ftoa(target) + `,"current_value":` + ftoa(current) + `}`
	if st := h.reqJSON("POST", "/v0/plans/"+itoa(p.ID)+"/goals/"+itoa(g.ID)+"/items", token, body, &it); st != http.StatusCreated {
		h.t.Fatalf("seed item: status %d", st)
	}
	return p.ID, g.ID, it.ID
}

func itoa(v int64) string  { return strconv.FormatInt(v, 10) }
func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("decode error envelope: %v; body=%s", err, body)
	}
	return e.Error.Code
}

// =====================================================================
// Local response DTOs (the httpapi DTOs are unexported).
// =====================================================================

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type planResp struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedBy int64  `json:"created_by"`
	Status    string `json:"status"`
}

type goalResp struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type itemResp struct {
	ID              int64    `json:"id"`
	GoalID          int64    `json:"goal_id"`
	Name            string   `json:"name"`
	Status          *string  `json:"status"`
	ItemType        *string  `json:"item_type"`
	TargetValue     *float64 `json:"target_value"`
	CurrentValue    *float64 `json:"current_value"`
	ProgressPercent *float64 `json:"progress_percent"`
}

type docResp struct {
	ID                   int64           `json:"id"`
	PlanItemID           int64           `json:"plan_item_id"`
	Title                string          `json:"title"`
	Status               string          `json:"status"`
	RecognizedText       string          `json:"recognized_text"`
	StructuredJSON       json.RawMessage `json:"structured_json"`
	RecognizedCategoryID *int64          `json:"recognized_category_id"`
	ModelVersion         *string         `json:"model_version"`
	OrganizationName     *string         `json:"organization_name"`
	INN                  *string         `json:"inn"`
	Description          *string         `json:"description"`
	FileName             string          `json:"file_name"`
	FilePath             string          `json:"file_path"`
	MimeType             string          `json:"mime_type"`
	FileSize             *int64          `json:"file_size"`
	ExternalNumber       *string         `json:"external_number"`
}

type storageResp struct {
	DocumentID       int64  `json:"document_id"`
	ObjectExists     bool   `json:"object_exists"`
	ObjectStatusCode int    `json:"object_status_code"`
	ObjectStatus     string `json:"object_status"`
	ObjectSize       *int64 `json:"object_size"`
}

type analyticsResp struct {
	LatestDocumentID     *int64   `json:"latest_document_id"`
	SourceDocumentsCount int      `json:"source_documents_count"`
	ProgressPercent      *float64 `json:"progress_percent"`
	Notes                []string `json:"notes"`
}

type analyzeResp struct {
	Recommendations string `json:"recommendations"`
}

type schemaColumn struct {
	Column string `json:"column"`
}

type queryResp struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int      `json:"row_count"`
}

type listMeta struct {
	Page       int `json:"page"`
	Size       int `json:"size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type listResp struct {
	Items json.RawMessage `json:"items"`
	Meta  listMeta        `json:"meta"`
}
