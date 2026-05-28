package httpapi

import (
	"net/http"

	"diplom.com/m/internal/usecase"
)

func (a *API) listPlans(w http.ResponseWriter, r *http.Request) {
	page, size := parsePage(r), parseSize(r)
	plans, total, err := a.Plans.List(r.Context(), userID(r), page, size)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	items := make([]planResponse, 0, len(plans))
	for _, p := range plans {
		items = append(items, toPlan(p))
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Meta: metaFor(page, size, total)})
}

func (a *API) createPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		Status      string  `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	plan, err := a.Plans.Create(r.Context(), userID(r), usecase.CreatePlanInput{
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPlanDetail(plan))
}
