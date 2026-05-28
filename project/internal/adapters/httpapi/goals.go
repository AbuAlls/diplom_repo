package httpapi

import (
	"net/http"

	"diplom.com/m/internal/usecase"
)

func (a *API) listGoals(w http.ResponseWriter, r *http.Request) {
	planID, ok := pathInt(r, "id_plan")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid plan id")
		return
	}
	page, size := parsePage(r), parseSize(r)
	goals, total, err := a.Goals.ListByPlan(r.Context(), userID(r), planID, page, size)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	items := make([]goalResponse, 0, len(goals))
	for _, g := range goals {
		items = append(items, toGoal(g))
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Meta: metaFor(page, size, total)})
}

func (a *API) createGoal(w http.ResponseWriter, r *http.Request) {
	planID, ok := pathInt(r, "id_plan")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid plan id")
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
		SortOrder   int     `json:"sort_order"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	goal, err := a.Goals.Create(r.Context(), userID(r), planID, usecase.CreateGoalInput{
		Name:        req.Name,
		Description: req.Description,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGoal(goal))
}
