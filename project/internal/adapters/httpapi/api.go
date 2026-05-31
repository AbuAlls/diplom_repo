package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"diplom.com/m/internal/usecase"
)

type API struct {
	Auth      *usecase.AuthService
	Plans     *usecase.PlanService
	Goals     *usecase.GoalService
	Items     *usecase.PlanItemService
	Docs      *usecase.DocumentService
	Analytics *usecase.AnalyticsService
	// InternalAnalytics backs the AI agent's read-only DB callbacks.
	InternalAnalytics *usecase.InternalAnalyticsService
	// InternalToken is the shared secret the AI service must present on
	// /api/* callbacks (the agent has no user JWT).
	InternalToken string
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", a.healthz)

	mux.HandleFunc("POST /v0/auth/token", a.token)
	mux.HandleFunc("POST /v0/auth/register", a.register)

	mux.Handle("GET /v0/plans", a.authenticated(a.listPlans))
	mux.Handle("POST /v0/plans", a.authenticated(a.createPlan))

	mux.Handle("GET /v0/plans/{id_plan}/goals", a.authenticated(a.listGoals))
	mux.Handle("POST /v0/plans/{id_plan}/goals", a.authenticated(a.createGoal))

	mux.Handle("GET /v0/plans/{id_plan}/goals/{id_goal}/items", a.authenticated(a.listItems))
	mux.Handle("POST /v0/plans/{id_plan}/goals/{id_goal}/items", a.authenticated(a.createItem))
	mux.Handle("PATCH /v0/plans/{id_plan}/goals/{id_goal}/items/{id_item}", a.authenticated(a.updateItem))

	// Both POST upload/{id_plan_item} and {id_document}/confirm share the
	// /v0/documents/{seg1}/{seg2} shape; the stdlib ServeMux can't disambiguate
	// them as separate wildcard patterns, so a single dispatcher handles both.
	mux.Handle("POST /v0/documents/{seg1}/{seg2}", a.authenticated(a.postDocumentAction))
	mux.Handle("GET /v0/documents", a.authenticated(a.listDocuments))
	mux.Handle("GET /v0/documents/{id_document}/download", a.authenticated(a.downloadDocument))
	mux.Handle("GET /v0/documents/{id_document}/storage", a.authenticated(a.getDocumentStorage))
	mux.Handle("GET /v0/documents/{id_document}", a.authenticated(a.getDocument))
	mux.Handle("PATCH /v0/documents/{id_document}", a.authenticated(a.patchDocument))

	mux.Handle("GET /v0/items/{id_item}/analytics", a.authenticated(a.getItemAnalytics))
	mux.Handle("POST /v0/items/{id_item}/analyze", a.authenticated(a.analyzeItem))

	// Internal callbacks for the AI analytics agent (shared-secret auth).
	mux.Handle("GET /api/schema", a.internalAuth(a.getSchema))
	mux.Handle("POST /api/analytics/query", a.internalAuth(a.runQuery))

	return mux
}

type ctxUserID struct{}

func (a *API) authenticated(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing bearer token")
			return
		}
		userID, err := a.Auth.ValidateAccessToken(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid access token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUserID{}, userID)))
	})
}

func userID(r *http.Request) int64 {
	v, _ := r.Context().Value(ctxUserID{}).(int64)
	return v
}

// internalAuth guards the AI-agent callbacks with a shared secret presented in
// the X-Internal-Token header (constant-time compared).
func (a *API) internalAuth(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Internal-Token")
		if a.InternalToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(a.InternalToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid internal token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
