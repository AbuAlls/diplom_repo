package httpapi

import (
	"net/http"

	"diplom.com/m/internal/ports"
)

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	planID, ok := pathInt(r, "id_plan")
	goalID, ok2 := pathInt(r, "id_goal")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	page, size := parsePage(r), parseSize(r)
	its, total, err := a.Items.ListByGoal(r.Context(), userID(r), planID, goalID, page, size)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	items := make([]planItemResponse, 0, len(its))
	for _, it := range its {
		items = append(items, toPlanItem(it))
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Meta: metaFor(page, size, total)})
}

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	planID, ok := pathInt(r, "id_plan")
	goalID, ok2 := pathInt(r, "id_goal")
	if !ok || !ok2 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	var req struct {
		Name         string   `json:"name"`
		Description  *string  `json:"description"`
		SortOrder    int      `json:"sort_order"`
		ItemType     string   `json:"item_type"`
		Status       string   `json:"status"`
		TargetValue  *float64 `json:"target_value"`
		CurrentValue *float64 `json:"current_value"`
		Unit         *string  `json:"unit"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	item, err := a.Items.Create(r.Context(), userID(r), planID, goalID, ports.PlanItemInput{
		Name:         req.Name,
		Description:  req.Description,
		SortOrder:    req.SortOrder,
		ItemType:     req.ItemType,
		Status:       req.Status,
		TargetValue:  req.TargetValue,
		CurrentValue: req.CurrentValue,
		Unit:         req.Unit,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toPlanItem(item))
}

func (a *API) updateItem(w http.ResponseWriter, r *http.Request) {
	planID, ok := pathInt(r, "id_plan")
	goalID, ok2 := pathInt(r, "id_goal")
	itemID, ok3 := pathInt(r, "id_item")
	if !ok || !ok2 || !ok3 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	var req struct {
		Name         *string  `json:"name"`
		Description  *string  `json:"description"`
		SortOrder    *int     `json:"sort_order"`
		ItemType     *string  `json:"item_type"`
		Status       *string  `json:"status"`
		TargetValue  *float64 `json:"target_value"`
		CurrentValue *float64 `json:"current_value"`
		Unit         *string  `json:"unit"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	patch := ports.PlanItemPatch{
		Name:         req.Name,
		Description:  req.Description,
		SortOrder:    req.SortOrder,
		ItemType:     req.ItemType,
		Status:       req.Status,
		TargetValue:  req.TargetValue,
		CurrentValue: req.CurrentValue,
		Unit:         req.Unit,
	}
	if patch == (ports.PlanItemPatch{}) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "At least one field is required")
		return
	}
	item, err := a.Items.Update(r.Context(), userID(r), planID, goalID, itemID, patch)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toPlanItem(item))
}
