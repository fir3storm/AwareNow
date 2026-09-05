package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/fir3storm/AwareNow/auth"
	"github.com/fir3storm/AwareNow/models"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User  models.User `json:"user"`
	Token string      `json:"token"`
}

func (as *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid request body"}, http.StatusBadRequest)
		return
	}

	user, err := models.GetUserByUsername(req.Username)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid username or password"}, http.StatusUnauthorized)
		return
	}

	if err := auth.ValidatePassword(req.Password, user.Hash); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid username or password"}, http.StatusUnauthorized)
		return
	}

	if user.AccountLocked {
		JSONResponse(w, models.Response{Success: false, Message: "Account is locked"}, http.StatusUnauthorized)
		return
	}

	user.LastLogin = time.Now().UTC()
	if err := models.PutUser(&user); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Failed to update login time"}, http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateJWT(user.Id, user.Username, user.Role.Name)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Failed to generate token"}, http.StatusInternalServerError)
		return
	}

	JSONResponse(w, models.Response{Success: true, Message: "Login successful", Data: loginResponse{User: user, Token: token}}, http.StatusOK)
}

func (as *Server) profile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int64)
	if !ok {
		JSONResponse(w, models.Response{Success: false, Message: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	user, err := models.GetUser(userID)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "User not found"}, http.StatusNotFound)
		return
	}

	JSONResponse(w, models.Response{Success: true, Message: "", Data: user}, http.StatusOK)
}
