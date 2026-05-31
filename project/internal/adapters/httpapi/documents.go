package httpapi

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"diplom.com/m/internal/ports"
	"diplom.com/m/internal/usecase"
)

const maxUploadBytes = 32 << 20 // 32 MiB

// postDocumentAction dispatches the two POST routes that share the
// /v0/documents/{seg1}/{seg2} shape, which the stdlib ServeMux cannot
// disambiguate when registered as separate wildcard patterns:
//   - /v0/documents/upload/{id_plan_item}
//   - /v0/documents/{id_document}/confirm
//   - /v0/documents/{id_document}/reject
//   - /v0/documents/{id_document}/reanalyze
func (a *API) postDocumentAction(w http.ResponseWriter, r *http.Request) {
	seg1 := r.PathValue("seg1")
	seg2 := r.PathValue("seg2")
	if seg1 == "upload" {
		itemID, ok := parseQueryInt(seg2)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
			return
		}
		a.uploadDocument(w, r, itemID)
		return
	}
	if seg2 == "confirm" {
		docID, ok := parseQueryInt(seg1)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
			return
		}
		a.confirmDocument(w, r, docID)
		return
	}
	if seg2 == "reject" {
		docID, ok := parseQueryInt(seg1)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
			return
		}
		a.rejectDocument(w, r, docID)
		return
	}
	if seg2 == "reanalyze" {
		docID, ok := parseQueryInt(seg1)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
			return
		}
		a.reanalyzeDocument(w, r, docID)
		return
	}
	writeError(w, http.StatusNotFound, "NOT_FOUND", "Unknown document action")
}

func (a *API) uploadDocument(w http.ResponseWriter, r *http.Request, itemID int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Missing file field")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	view, err := a.Docs.Upload(r.Context(), userID(r), itemID, header.Filename, mimeType, file)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocument(view))
}

func (a *API) listDocuments(w http.ResponseWriter, r *http.Request) {
	var planItemID *int64
	if v := r.URL.Query().Get("plan_item_id"); v != "" {
		id, ok := parseQueryInt(v)
		if !ok {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid plan_item_id")
			return
		}
		planItemID = &id
	}
	page, size := parsePage(r), parseSize(r)
	views, total, err := a.Docs.List(r.Context(), userID(r), planItemID, page, size)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	items := make([]documentResponse, 0, len(views))
	for _, v := range views {
		items = append(items, toDocument(v))
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Meta: metaFor(page, size, total)})
}

func (a *API) getDocument(w http.ResponseWriter, r *http.Request) {
	docID, ok := pathInt(r, "id_document")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	view, err := a.Docs.Get(r.Context(), userID(r), docID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocument(view))
}

func (a *API) getDocumentStorage(w http.ResponseWriter, r *http.Request) {
	docID, ok := pathInt(r, "id_document")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	status, err := a.Docs.StorageStatus(r.Context(), userID(r), docID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocumentStorage(status))
}

func (a *API) downloadDocument(w http.ResponseWriter, r *http.Request) {
	docID, ok := pathInt(r, "id_document")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	download, err := a.Docs.Download(r.Context(), userID(r), docID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	defer download.Object.Body.Close()

	contentType := download.Object.Status.ContentType
	if contentType == "" {
		contentType = download.Doc.MimeType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": download.Doc.FileName}))
	if download.Object.Status.ContentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*download.Object.Status.ContentLength, 10))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, download.Object.Body)
}

func (a *API) patchDocument(w http.ResponseWriter, r *http.Request) {
	docID, ok := pathInt(r, "id_document")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	var raw map[string]json.RawMessage
	if !decodeJSON(w, r, &raw) {
		return
	}
	if len(raw) == 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "At least one field is required")
		return
	}

	upd := usecase.DocumentUpdate{}
	if v, ok := raw["recognized_category_id"]; ok {
		upd.SetCategory = true
		if err := json.Unmarshal(v, &upd.RecognizedCategoryID); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid recognized_category_id")
			return
		}
	}

	patch := ports.DocumentPatch{}
	if !unmarshalString(w, raw, "external_number", &patch.ExternalNumber) ||
		!unmarshalString(w, raw, "organization_name", &patch.OrganizationName) ||
		!unmarshalString(w, raw, "inn", &patch.INN) ||
		!unmarshalString(w, raw, "description", &patch.Description) {
		return
	}
	if !unmarshalStrings(w, raw, "personal_data", &patch.PersonalData) ||
		!unmarshalStrings(w, raw, "organization_data", &patch.OrganizationData) ||
		!unmarshalStrings(w, raw, "prices", &patch.Prices) ||
		!unmarshalStrings(w, raw, "product_names", &patch.ProductNames) ||
		!unmarshalStrings(w, raw, "contract_numbers", &patch.ContractNumbers) {
		return
	}
	if v, ok := raw["quantities"]; ok {
		if err := json.Unmarshal(v, &patch.Quantities); err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid quantities")
			return
		}
	}
	if v, ok := raw["document_date"]; ok {
		t, ok := parseDate(w, v)
		if !ok {
			return
		}
		patch.DocumentDate = t
	}
	if v, ok := raw["deadlines"]; ok {
		ts, ok := parseDates(w, v)
		if !ok {
			return
		}
		patch.Deadlines = ts
	}
	upd.Fields = patch

	view, err := a.Docs.Patch(r.Context(), userID(r), docID, upd)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocument(view))
}

func (a *API) confirmDocument(w http.ResponseWriter, r *http.Request, docID int64) {
	view, err := a.Docs.Confirm(r.Context(), userID(r), docID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocument(view))
}

func (a *API) rejectDocument(w http.ResponseWriter, r *http.Request, docID int64) {
	view, err := a.Docs.Reject(r.Context(), userID(r), docID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocument(view))
}

func (a *API) reanalyzeDocument(w http.ResponseWriter, r *http.Request, docID int64) {
	view, err := a.Docs.Reanalyze(r.Context(), userID(r), docID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDocument(view))
}

func unmarshalString(w http.ResponseWriter, raw map[string]json.RawMessage, key string, dst **string) bool {
	v, ok := raw[key]
	if !ok {
		return true
	}
	if err := json.Unmarshal(v, dst); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid "+key)
		return false
	}
	return true
}

func unmarshalStrings(w http.ResponseWriter, raw map[string]json.RawMessage, key string, dst *[]string) bool {
	v, ok := raw[key]
	if !ok {
		return true
	}
	if err := json.Unmarshal(v, dst); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid "+key)
		return false
	}
	return true
}

func parseDate(w http.ResponseWriter, v json.RawMessage) (*time.Time, bool) {
	var s *string
	if err := json.Unmarshal(v, &s); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid date")
		return nil, false
	}
	if s == nil {
		return nil, true
	}
	t, err := time.Parse(dateLayout, *s)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid date format, expected YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}

func parseDates(w http.ResponseWriter, v json.RawMessage) ([]time.Time, bool) {
	var ss []string
	if err := json.Unmarshal(v, &ss); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid deadlines")
		return nil, false
	}
	out := make([]time.Time, 0, len(ss))
	for _, s := range ss {
		t, err := time.Parse(dateLayout, s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid deadline date, expected YYYY-MM-DD")
			return nil, false
		}
		out = append(out, t)
	}
	return out, true
}
