package httpapi

import "net/http"

// analyzeItem (authenticated) runs the analytics agent for an owned plan item.
func (a *API) analyzeItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathInt(r, "id_item")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	var req analyzeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	rec, err := a.Analytics.Recommendations(r.Context(), userID(r), itemID, req.Model, req.Message)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, analyzeResponse{Recommendations: rec})
}

// getSchema (internal) returns the public schema for the AI agent.
func (a *API) getSchema(w http.ResponseWriter, r *http.Request) {
	schema, err := a.InternalAnalytics.Schema(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toSchemaResponse(schema))
}

// runQuery (internal) executes a read-only SELECT on behalf of the AI agent.
// ownerID is extracted from context when a per-session token was presented;
// it is 0 when the global token is used, in which case no row scoping applies.
func (a *API) runQuery(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := r.Context().Value(ctxInternalOwnerID{}).(int64)

	var req queryRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	cols, rows, count, err := a.InternalAnalytics.RunQuery(r.Context(), req.Query, ownerID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	if rows == nil {
		rows = [][]any{}
	}
	writeJSON(w, http.StatusOK, queryResponse{Columns: cols, Rows: rows, RowCount: count})
}
