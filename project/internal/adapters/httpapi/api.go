package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"diplom.com/m/internal/auth"
	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
	"diplom.com/m/internal/usecase"
)

type API struct {
	Auth     *usecase.AuthService
	Docs     *usecase.DocumentService
	DocRepo  ports.DocumentRepo
	JobRepo  ports.JobRepo
	Analysis ports.AnalysisRepo
	Store    ports.ObjectStore
	SSE      *SSEHandler
}

func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("POST /auth/register", a.register)
	mux.HandleFunc("POST /auth/login", a.login)
	mux.HandleFunc("POST /auth/refresh", a.refresh)
	mux.HandleFunc("POST /auth/logout", a.logout)

	mux.Handle("GET /jobs/", a.authenticated(http.HandlerFunc(a.sseByPath)))
	mux.Handle("POST /documents", a.authenticated(http.HandlerFunc(a.uploadDocument)))
	mux.Handle("GET /documents", a.authenticated(http.HandlerFunc(a.listDocuments)))
	mux.Handle("GET /documents/", a.authenticated(http.HandlerFunc(a.documentByPath)))
	return mux
}

func (a *API) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		userID, err := a.Auth.ValidateAccessToken(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUserID{}, userID)))
	})
}

type ctxUserID struct{}

func userIDFromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(ctxUserID{}).(string)
	if !ok || v == "" {
		return "", errors.New("unauthorized")
	}
	return v, nil
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.Auth.Register(r.Context(), req.Email, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.setRefreshCookie(w, res)
	writeJSON(w, http.StatusCreated, map[string]any{"access_token": res.AccessToken})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.Auth.Login(r.Context(), req.Email, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	a.setRefreshCookie(w, res)
	writeJSON(w, http.StatusOK, map[string]any{"access_token": res.AccessToken})
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		http.Error(w, "refresh cookie is required", http.StatusUnauthorized)
		return
	}
	res, err := a.Auth.Refresh(r.Context(), cookie.Value, r.UserAgent(), clientIP(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	a.setRefreshCookie(w, res)
	writeJSON(w, http.StatusOK, map[string]any{"access_token": res.AccessToken})
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("refresh_token"); err == nil {
		_ = a.Auth.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   isSecureCookie(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) uploadDocument(w http.ResponseWriter, r *http.Request) {
	ownerID, err := userIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	mime := hdr.Header.Get("Content-Type")
	if mime == "" {
		mime = "application/octet-stream"
	}
	docID, jobID, err := a.Docs.Upload(r.Context(), ownerID, hdr.Filename, mime, file, hdr.Size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"document_id": docID, "job_id": jobID})
}

func (a *API) listDocuments(w http.ResponseWriter, r *http.Request) {
	ownerID, err := userIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	cursor := r.URL.Query().Get("cursor")
	var status *domain.DocStatus
	if st := r.URL.Query().Get("status"); st != "" {
		s := domain.DocStatus(st)
		status = &s
	}
	items, next, err := a.DocRepo.List(r.Context(), ownerID, limit, cursor, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

func (a *API) documentByPath(w http.ResponseWriter, r *http.Request) {
	ownerID, err := userIDFromContext(r.Context())
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/documents/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	docID := parts[0]
	if len(parts) == 1 {
		a.getDocument(w, r, ownerID, docID)
		return
	}
	switch parts[1] {
	case "download":
		a.downloadDocument(w, r, ownerID, docID)
	case "results":
		a.documentResults(w, r, ownerID, docID)
	default:
		http.NotFound(w, r)
	}
}

func (a *API) getDocument(w http.ResponseWriter, r *http.Request, ownerID, docID string) {
	doc, err := a.DocRepo.Get(r.Context(), ownerID, docID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (a *API) downloadDocument(w http.ResponseWriter, r *http.Request, ownerID, docID string) {
	doc, err := a.DocRepo.Get(r.Context(), ownerID, docID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	rc, err := a.Store.Get(r.Context(), doc.ObjectKey)
	if err != nil {
		http.Error(w, "object not found", http.StatusNotFound)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", doc.Mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+doc.Filename+"\"")
	_, _ = io.Copy(w, rc)
}

func (a *API) documentResults(w http.ResponseWriter, r *http.Request, ownerID, docID string) {
	fields, err := a.Analysis.GetExtraction(r.Context(), ownerID, docID)
	if err != nil {
		http.Error(w, "results not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"document_id": docID, "fields": fields})
}

func (a *API) sseByPath(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/events") {
		http.NotFound(w, r)
		return
	}
	a.SSE.ServeHTTP(w, r)
}

func (a *API) setRefreshCookie(w http.ResponseWriter, res usecase.AuthResult) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    auth.BuildRefreshCookieValue(res.RefreshSessionID, res.RefreshTokenPlain),
		Path:     "/auth/refresh",
		HttpOnly: true,
		Secure:   isSecureCookie(),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().UTC().Add(7 * 24 * time.Hour),
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func clientIP(r *http.Request) string {
	h := r.Header.Get("X-Forwarded-For")
	if h != "" {
		parts := strings.Split(h, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}

func isSecureCookie() bool {
	return strings.EqualFold(strings.TrimSpace(getenv("PP_ENV", "dev")), "prod")
}

func getenv(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}
