package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"diplom.com/m/internal/usecase"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	Size       int `json:"size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorPayload{Code: code, Message: message}})
}

// writeUsecaseError maps domain/use-case sentinel errors to HTTP responses.
func writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request")
	case errors.Is(err, usecase.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid credentials")
	case errors.Is(err, usecase.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN", "Access denied")
	case errors.Is(err, usecase.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Entity not found")
	case errors.Is(err, usecase.ErrConflict):
		writeError(w, http.StatusConflict, "CONFLICT", "Action not allowed for current state")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body")
		return false
	}
	return true
}

func parsePage(r *http.Request) int {
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v >= 1 {
		return v
	}
	return 1
}

func parseSize(r *http.Request) int {
	v, err := strconv.Atoi(r.URL.Query().Get("size"))
	if err != nil || v < 1 {
		return 20
	}
	if v > 100 {
		return 100
	}
	return v
}

func metaFor(page, size, total int) PaginationMeta {
	pages := 0
	if size > 0 {
		pages = (total + size - 1) / size
	}
	return PaginationMeta{Page: page, Size: size, Total: total, TotalPages: pages}
}

func parseQueryInt(v string) (int64, bool) {
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func pathInt(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
