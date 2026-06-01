package httpapi

import (
	"net/http"

	"diplom.com/m/internal/usecase"
)

func (a *API) listGroups(w http.ResponseWriter, r *http.Request) {
	page, size := parsePage(r), parseSize(r)
	groups, total, err := a.Groups.List(r.Context(), userID(r), page, size)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	items := make([]groupResponse, 0, len(groups))
	for _, g := range groups {
		items = append(items, toGroup(g))
	}
	writeJSON(w, http.StatusOK, listResponse{Items: items, Meta: metaFor(page, size, total)})
}

func (a *API) createGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	g, err := a.Groups.Create(r.Context(), userID(r), usecase.CreateGroupInput{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGroup(g))
}

func (a *API) getGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathInt(r, "id_group")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	g, members, err := a.Groups.Get(r.Context(), userID(r), groupID)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toGroupDetail(g, members))
}

func (a *API) addGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathInt(r, "id_group")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	m, err := a.Groups.AddMemberByEmail(r.Context(), userID(r), groupID, req.Email)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toGroupMember(m))
}

func (a *API) removeGroupMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathInt(r, "id_group")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid path id")
		return
	}
	memberID, ok := pathInt(r, "id_member")
	if !ok {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid member id")
		return
	}
	if err := a.Groups.RemoveMember(r.Context(), userID(r), groupID, memberID); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
