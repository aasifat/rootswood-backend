package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"rootswood/config"
	"rootswood/models"
	"rootswood/utils"

	"golang.org/x/crypto/bcrypt"
)

func HandleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.JsonErr(w, "Bad request", 400)
		return
	}
	if body.Name == "" || body.Email == "" || body.Password == "" {
		utils.JsonErr(w, "All fields are required", 400)
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	var id int64
	err := config.DB.QueryRow(
		`INSERT INTO users(name,email,password) VALUES($1,$2,$3) RETURNING id`,
		body.Name, strings.ToLower(body.Email), string(hash),
	).Scan(&id)
	if err != nil {
		utils.JsonErr(w, "Email already registered", 409)
		return
	}
	u := models.User{ID: id, Name: body.Name, Email: body.Email}
	token, _ := config.SignToken(u)
	w.WriteHeader(http.StatusCreated)
	utils.JsonOK(w, map[string]interface{}{"token": token, "user": u})
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.JsonErr(w, "Bad request", 400)
		return
	}
	var u models.User
	var hash string
	err := config.DB.QueryRow(
		`SELECT id,name,email,password FROM users WHERE email=$1`,
		strings.ToLower(body.Email),
	).Scan(&u.ID, &u.Name, &u.Email, &hash)
	if err != nil {
		utils.JsonErr(w, "Invalid credentials", 401)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		utils.JsonErr(w, "Invalid credentials", 401)
		return
	}
	token, _ := config.SignToken(u)
	utils.JsonOK(w, map[string]interface{}{"token": token, "user": u})
}

func HandleMe(w http.ResponseWriter, r *http.Request) {
	uid := utils.UserIDFromReq(r)
	var u models.User
	err := config.DB.QueryRow(
		`SELECT id,name,email,created_at FROM users WHERE id=$1`, uid,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		utils.JsonErr(w, "User not found", 404)
		return
	}
	utils.JsonOK(w, u)
}

func HandleUpdateMe(w http.ResponseWriter, r *http.Request) {
	uid := utils.UserIDFromReq(r)
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name != "" {
		config.DB.Exec(`UPDATE users SET name=$1 WHERE id=$2`, body.Name, uid)
	}
	if body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
		config.DB.Exec(`UPDATE users SET password=$1 WHERE id=$2`, string(hash), uid)
	}
	HandleMe(w, r)
}
