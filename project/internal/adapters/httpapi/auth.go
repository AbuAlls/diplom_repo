package httpapi

import (
	"net/http"

	"diplom.com/m/internal/usecase"
)

func (a *API) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid form body")
		return
	}
	username := r.PostForm.Get("username")
	password := r.PostForm.Get("password")
	if username == "" || password == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "username and password are required")
		return
	}
	res, err := a.Auth.Token(r.Context(), username, password)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeTokenResponse(w, http.StatusOK, res)
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"full_name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.Auth.Register(r.Context(), req.Email, req.Password, req.FullName)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeTokenResponse(w, http.StatusCreated, res)
}

func writeTokenResponse(w http.ResponseWriter, status int, res usecase.AuthResult) {
	writeJSON(w, status, tokenResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    res.ExpiresIn,
		TokenType:    res.TokenType,
	})
}
