package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"testing"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

// --- in-memory repos for the documents milestone ---

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
	for _, d := range m.byID {
		if d.UploadedBy != ownerID {
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
	saved map[string]int64
}

func newMemFileStore() *memFileStore { return &memFileStore{saved: map[string]int64{}} }

func (m *memFileStore) Save(_ context.Context, key string, r io.Reader) (int64, error) {
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		return 0, err
	}
	m.saved[key] = n
	return n, nil
}

// --- helpers ---

// seedItem creates a plan→goal→item tree owned by token and returns the item id (1).
func seedItem(t *testing.T, base, token string) {
	t.Helper()
	resp := do(t, "POST", base+"/v0/plans", token, `{"name":"Plan"}`)
	resp.Body.Close()
	resp = do(t, "POST", base+"/v0/plans/1/goals", token, `{"name":"Goal"}`)
	resp.Body.Close()
	resp = do(t, "POST", base+"/v0/plans/1/goals/1/items", token, `{"name":"Item","target_value":100,"current_value":40}`)
	resp.Body.Close()
}

func uploadDoc(t *testing.T, base, token string, itemID int64, fileName, content string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	mw.Close()

	url := base + "/v0/documents/upload/" + strconv.FormatInt(itemID, 10)
	r, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	return resp
}

// --- tests ---

func TestDocumentUploadHappyPath(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "doc@example.com")
	seedItem(t, srv.URL, token)

	resp := uploadDoc(t, srv.URL, token, 1, "contract.pdf", "hello bytes")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}
	var doc documentResponse
	json.NewDecoder(resp.Body).Decode(&doc)
	if doc.ID == 0 || doc.PlanItemID != 1 {
		t.Fatalf("unexpected doc: %+v", doc)
	}
	if doc.Status != "pending_review" {
		t.Fatalf("expected pending_review, got %q", doc.Status)
	}
	if doc.RecognizedText == "" {
		t.Fatalf("expected non-empty recognized_text")
	}
	if doc.ModelVersion == nil || *doc.ModelVersion != "mock-v0" {
		t.Fatalf("expected model_version mock-v0, got %v", doc.ModelVersion)
	}
	if doc.FileSize == nil || *doc.FileSize != int64(len("hello bytes")) {
		t.Fatalf("expected file_size %d, got %v", len("hello bytes"), doc.FileSize)
	}
	var structured map[string]any
	if err := json.Unmarshal(doc.StructuredJSON, &structured); err != nil {
		t.Fatalf("structured_json not an object: %v", err)
	}
	if structured["mock"] != true {
		t.Fatalf("expected mock=true in structured_json, got %+v", structured)
	}
}

func TestDocumentListGetPatchConfirmFlow(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "doc@example.com")
	seedItem(t, srv.URL, token)
	uploadDoc(t, srv.URL, token, 1, "doc.pdf", "data").Body.Close()

	// list
	resp := do(t, "GET", srv.URL+"/v0/documents?plan_item_id=1", token, "")
	var list listResponse
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Meta.Total != 1 {
		t.Fatalf("expected total 1, got %d", list.Meta.Total)
	}

	// get
	resp = do(t, "GET", srv.URL+"/v0/documents/1", token, "")
	var doc documentResponse
	json.NewDecoder(resp.Body).Decode(&doc)
	resp.Body.Close()
	if doc.ID != 1 {
		t.Fatalf("expected doc id 1, got %d", doc.ID)
	}

	// patch: core field + analysis category
	resp = do(t, "PATCH", srv.URL+"/v0/documents/1", token, `{"organization_name":"Acme","recognized_category_id":7}`)
	json.NewDecoder(resp.Body).Decode(&doc)
	resp.Body.Close()
	if doc.OrganizationName == nil || *doc.OrganizationName != "Acme" {
		t.Fatalf("expected organization_name Acme, got %v", doc.OrganizationName)
	}
	if doc.RecognizedCategoryID == nil || *doc.RecognizedCategoryID != 7 {
		t.Fatalf("expected recognized_category_id 7, got %v", doc.RecognizedCategoryID)
	}

	// confirm
	resp = do(t, "POST", srv.URL+"/v0/documents/1/confirm", token, "")
	json.NewDecoder(resp.Body).Decode(&doc)
	resp.Body.Close()
	if doc.Status != "confirmed" {
		t.Fatalf("expected confirmed, got %q", doc.Status)
	}

	// second confirm → 409
	resp = do(t, "POST", srv.URL+"/v0/documents/1/confirm", token, "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on second confirm, got %d", resp.StatusCode)
	}
}

func TestDocumentPatchEmptyBody(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "doc@example.com")
	seedItem(t, srv.URL, token)
	uploadDoc(t, srv.URL, token, 1, "doc.pdf", "data").Body.Close()

	resp := do(t, "PATCH", srv.URL+"/v0/documents/1", token, `{}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on empty patch, got %d", resp.StatusCode)
	}
}

func TestItemAnalytics(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "doc@example.com")
	seedItem(t, srv.URL, token)
	uploadDoc(t, srv.URL, token, 1, "doc.pdf", "data").Body.Close()

	resp := do(t, "GET", srv.URL+"/v0/items/1/analytics", token, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("analytics status = %d", resp.StatusCode)
	}
	var a itemAnalyticsResponse
	json.NewDecoder(resp.Body).Decode(&a)
	if a.SourceDocumentsCount != 1 {
		t.Fatalf("expected source_documents_count 1, got %d", a.SourceDocumentsCount)
	}
	if a.LatestDocumentID == nil || *a.LatestDocumentID != 1 {
		t.Fatalf("expected latest_document_id 1, got %v", a.LatestDocumentID)
	}
	if a.ProgressPercent == nil || *a.ProgressPercent != 40 {
		t.Fatalf("expected progress 40, got %v", a.ProgressPercent)
	}
}

func TestDocumentUnauthorized(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	resp := do(t, "GET", srv.URL+"/v0/documents", "", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestDocumentForbiddenOtherUser(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	tokenA := register(t, srv.URL, "a@example.com")
	tokenB := register(t, srv.URL, "b@example.com")
	seedItem(t, srv.URL, tokenA)
	uploadDoc(t, srv.URL, tokenA, 1, "doc.pdf", "data").Body.Close()

	resp := do(t, "GET", srv.URL+"/v0/documents/1", tokenB, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDocumentNotFound(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "doc@example.com")
	resp := do(t, "GET", srv.URL+"/v0/documents/999", token, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
