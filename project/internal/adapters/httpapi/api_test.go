package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"diplom.com/m/internal/adapters/recognition"
	"diplom.com/m/internal/auth"
	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"diplom.com/m/internal/usecase"
)

// --- in-memory repos ---

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
	nextID int64
}

func newMemPlanRepo() *memPlanRepo { return &memPlanRepo{byID: map[int64]domain.Plan{}} }

func (m *memPlanRepo) Create(_ context.Context, ownerID int64, name string, desc *string, status string) (domain.Plan, error) {
	m.nextID++
	p := domain.Plan{ID: m.nextID, Name: name, Description: desc, CreatedBy: ownerID, Status: status, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.byID[p.ID] = p
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
	var all []domain.Plan
	for _, p := range m.byID {
		if p.CreatedBy == ownerID {
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
	it := m.byID[id]
	if patch.CurrentValue != nil {
		it.CurrentValue = patch.CurrentValue
	}
	if patch.TargetValue != nil {
		it.TargetValue = patch.TargetValue
	}
	if patch.Name != nil {
		it.Name = *patch.Name
	}
	m.byID[id] = it
	return it, nil
}

// --- test harness ---

func newTestServer() (*httptest.Server, *memPlanRepo) {
	users := newMemUserRepo()
	plans := newMemPlanRepo()
	goals := newMemGoalRepo()
	items := newMemItemRepo()

	folders := newMemFolderRepo()
	docs := newMemDocumentRepo()
	extracted := newMemExtractedRepo()
	store := newMemFileStore()
	recognizer := recognition.New()

	tm := auth.TokenManager{Secret: []byte("test-secret"), Issuer: "test"}
	authSvc := &usecase.AuthService{Users: users, Tokens: tm, AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour}
	api := &API{
		Auth:  authSvc,
		Plans: &usecase.PlanService{Plans: plans},
		Goals: &usecase.GoalService{Goals: goals, Plans: plans},
		Items: &usecase.PlanItemService{Items: items, Goals: goals, Plans: plans},
		Docs: &usecase.DocumentService{
			Docs: docs, Folders: folders, Items: items, Goals: goals, Plans: plans,
			Store: store, Extracted: extracted, Recognizer: recognizer,
		},
		Analytics: &usecase.AnalyticsService{
			Items: items, Goals: goals, Plans: plans, Docs: docs,
			Analyzer: &fakeAnalyzer{}, Sessions: usecase.NewSessionStore(),
		},
		InternalAnalytics: &usecase.InternalAnalyticsService{Query: &fakeQueryRepo{}},
		InternalToken:     testInternalToken,
		Sessions:          usecase.NewSessionStore(),
	}
	return httptest.NewServer(api.Routes()), plans
}

func register(t *testing.T, base, email string) string {
	t.Helper()
	body := `{"email":"` + email + `","password":"password123","full_name":"Test User"}`
	resp, err := http.Post(base+"/v0/auth/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register status = %d", resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if tr.AccessToken == "" || tr.TokenType != "Bearer" {
		t.Fatalf("bad token response: %+v", tr)
	}
	return tr.AccessToken
}

func do(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	var r *http.Request
	var err error
	if body == "" {
		r, err = http.NewRequest(method, url, nil)
	} else {
		r, err = http.NewRequest(method, url, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, url, err)
	}
	return resp
}

func TestAuthAndPlanTreeHappyPath(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "user@example.com")

	// token endpoint (password grant)
	form := url.Values{"grant_type": {"password"}, "username": {"user@example.com"}, "password": {"password123"}}
	resp, err := http.PostForm(srv.URL+"/v0/auth/token", form)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token status = %d", resp.StatusCode)
	}

	// create plan
	resp = do(t, "POST", srv.URL+"/v0/plans", token, `{"name":"Plan A"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create plan status = %d", resp.StatusCode)
	}
	var plan planDetailResponse
	json.NewDecoder(resp.Body).Decode(&plan)
	resp.Body.Close()
	if plan.ID == 0 || plan.Status != "active" {
		t.Fatalf("unexpected plan: %+v", plan)
	}

	// list plans
	resp = do(t, "GET", srv.URL+"/v0/plans", token, "")
	var list listResponse
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if list.Meta.Total != 1 {
		t.Fatalf("expected total 1, got %d", list.Meta.Total)
	}

	// create goal
	resp = do(t, "POST", srv.URL+"/v0/plans/1/goals", token, `{"name":"Goal A"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create goal status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// create item
	resp = do(t, "POST", srv.URL+"/v0/plans/1/goals/1/items", token, `{"name":"Item A","target_value":100,"current_value":50}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create item status = %d", resp.StatusCode)
	}
	var item planItemResponse
	json.NewDecoder(resp.Body).Decode(&item)
	resp.Body.Close()
	if item.ProgressPercent == nil || *item.ProgressPercent != 50 {
		t.Fatalf("expected progress 50, got %+v", item.ProgressPercent)
	}

	// patch item
	resp = do(t, "PATCH", srv.URL+"/v0/plans/1/goals/1/items/1", token, `{"current_value":100}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch item status = %d", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&item)
	resp.Body.Close()
	if item.ProgressPercent == nil || *item.ProgressPercent != 100 {
		t.Fatalf("expected progress 100 after patch, got %+v", item.ProgressPercent)
	}
}

func TestUnauthorizedWithoutToken(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	resp := do(t, "GET", srv.URL+"/v0/plans", "", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestForbiddenOtherUsersPlan(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	tokenA := register(t, srv.URL, "a@example.com")
	tokenB := register(t, srv.URL, "b@example.com")

	resp := do(t, "POST", srv.URL+"/v0/plans", tokenA, `{"name":"A's plan"}`)
	resp.Body.Close()

	// B tries to add goal to A's plan (id 1)
	resp = do(t, "POST", srv.URL+"/v0/plans/1/goals", tokenB, `{"name":"intrusion"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestNotFoundMissingPlan(t *testing.T) {
	srv, _ := newTestServer()
	defer srv.Close()
	token := register(t, srv.URL, "user@example.com")
	resp := do(t, "GET", srv.URL+"/v0/plans/999/goals", token, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
