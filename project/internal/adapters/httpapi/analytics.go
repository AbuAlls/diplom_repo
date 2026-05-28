package httpapi

import "net/http"

func (a *API) getItemAnalytics(w http.ResponseWriter, r *http.Request) {
	itemID, ok := pathInt(r, "id_item")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	res, err := a.Analytics.ItemAnalytics(r.Context(), userID(r), itemID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toItemAnalytics(res))
}
