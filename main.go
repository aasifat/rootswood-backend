package main

// ─────────────────────────────────────────────────────────────────────────────
//  Rootswood Backend  ·  Go 1.22+
//  Single-file server with JWT auth, PostgreSQL persistence, full CRUD.
//
//  Dependencies (go get):
//    github.com/golang-jwt/jwt/v5
//    github.com/lib/pq
//    golang.org/x/crypto
//
//  Run:
//    DATABASE_URL=postgres://... go run main.go
//
//  API base: http://localhost:8080/api
// ─────────────────────────────────────────────────────────────────────────────

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// ── Config ────────────────────────────────────────────────────────────────────

var jwtSecret = []byte(envOr("JWT_SECRET", "rootswood-secret-change-in-prod"))
var db *sql.DB

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ── Models ────────────────────────────────────────────────────────────────────

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type FamilyTree struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	MemberCount int       `json:"member_count"`
}

type Member struct {
	ID         int64   `json:"id"`
	TreeID     int64   `json:"tree_id"`
	Name       string  `json:"name"`
	Born       int     `json:"born"`
	Died       *int    `json:"died"`
	Gender     string  `json:"gender"`
	Relation   string  `json:"relation"`
	Location   string  `json:"location"`
	Bio        string  `json:"bio"`
	Avatar     string  `json:"avatar"`
	Color      string  `json:"color"`
	Generation int     `json:"generation"`
	PositionX  float64 `json:"position_x"`
	PositionY  float64 `json:"position_y"`
}

type Relationship struct {
	ID      int64  `json:"id"`
	TreeID  int64  `json:"tree_id"`
	FromID  int64  `json:"from_id"`
	ToID    int64  `json:"to_id"`
	RelType string `json:"rel_type"` // parent | spouse | sibling
	Label   string `json:"label"`
}

type Claims struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

// ── Database setup ────────────────────────────────────────────────────────────

func initDB() {
	var err error
	db, err = sql.Open("postgres", envOr("DATABASE_URL", "postgres://localhost/rootswood?sslmode=disable"))
	if err != nil {
		log.Fatal("DB open:", err)
	}
	if err = db.Ping(); err != nil {
		log.Fatal("DB ping:", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id         SERIAL PRIMARY KEY,
		name       TEXT      NOT NULL,
		email      TEXT      NOT NULL UNIQUE,
		password   TEXT      NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS family_trees (
		id          SERIAL PRIMARY KEY,
		user_id     INTEGER   NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name        TEXT      NOT NULL,
		description TEXT      DEFAULT '',
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS members (
		id          SERIAL PRIMARY KEY,
		tree_id     INTEGER   NOT NULL REFERENCES family_trees(id) ON DELETE CASCADE,
		name        TEXT      NOT NULL,
		born        INTEGER   DEFAULT 0,
		died        INTEGER,
		gender      TEXT      DEFAULT 'other',
		relation    TEXT      DEFAULT '',
		location    TEXT      DEFAULT '',
		bio         TEXT      DEFAULT '',
		avatar      TEXT      DEFAULT '',
		color       TEXT      DEFAULT '#4A7C59',
		generation  INTEGER   DEFAULT 0,
		position_x  FLOAT     DEFAULT 0,
		position_y  FLOAT     DEFAULT 0,
		created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS relationships (
		id       SERIAL PRIMARY KEY,
		tree_id  INTEGER NOT NULL REFERENCES family_trees(id) ON DELETE CASCADE,
		from_id  INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		to_id    INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		rel_type TEXT    NOT NULL,
		label    TEXT    DEFAULT '',
		UNIQUE(from_id, to_id, rel_type)
	);`

	if _, err = db.Exec(schema); err != nil {
		log.Fatal("Schema:", err)
	}
	log.Println("✅ Database ready")
}

// ── JWT helpers ───────────────────────────────────────────────────────────────

func signToken(u User) (string, error) {
	claims := Claims{
		UserID: u.ID,
		Email:  u.Email,
		Name:   u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func parseBearerToken(r *http.Request) (*Claims, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return token.Claims.(*Claims), nil
}

// ── Middleware ────────────────────────────────────────────────────────────────

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := parseBearerToken(r)
		if err != nil {
			jsonErr(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		r.Header.Set("X-User-ID", strconv.FormatInt(claims.UserID, 10))
		r.Header.Set("X-User-Name", claims.Name)
		next(w, r)
	}
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Response helpers ──────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func userIDFromReq(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
	return id
}

// ── Auth handlers ─────────────────────────────────────────────────────────────

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "Bad request", 400)
		return
	}
	if body.Name == "" || body.Email == "" || body.Password == "" {
		jsonErr(w, "All fields are required", 400)
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	var id int64
	err := db.QueryRow(
		`INSERT INTO users(name,email,password) VALUES($1,$2,$3) RETURNING id`,
		body.Name, strings.ToLower(body.Email), string(hash),
	).Scan(&id)
	if err != nil {
		jsonErr(w, "Email already registered", 409)
		return
	}
	u := User{ID: id, Name: body.Name, Email: body.Email}
	token, _ := signToken(u)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]interface{}{"token": token, "user": u})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "Bad request", 400)
		return
	}
	var u User
	var hash string
	err := db.QueryRow(
		`SELECT id,name,email,password FROM users WHERE email=$1`,
		strings.ToLower(body.Email),
	).Scan(&u.ID, &u.Name, &u.Email, &hash)
	if err != nil {
		jsonErr(w, "Invalid credentials", 401)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		jsonErr(w, "Invalid credentials", 401)
		return
	}
	token, _ := signToken(u)
	jsonOK(w, map[string]interface{}{"token": token, "user": u})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromReq(r)
	var u User
	err := db.QueryRow(
		`SELECT id,name,email,created_at FROM users WHERE id=$1`, uid,
	).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt)
	if err != nil {
		jsonErr(w, "User not found", 404)
		return
	}
	jsonOK(w, u)
}

func handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromReq(r)
	var body struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name != "" {
		db.Exec(`UPDATE users SET name=$1 WHERE id=$2`, body.Name, uid)
	}
	if body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
		db.Exec(`UPDATE users SET password=$1 WHERE id=$2`, string(hash), uid)
	}
	handleMe(w, r)
}

// ── Family Tree handlers ──────────────────────────────────────────────────────

func handleListTrees(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromReq(r)
	rows, err := db.Query(`
		SELECT t.id,t.user_id,t.name,t.description,t.created_at,t.updated_at,
		       COUNT(m.id) as member_count
		FROM family_trees t
		LEFT JOIN members m ON m.tree_id=t.id
		WHERE t.user_id=$1
		GROUP BY t.id
		ORDER BY t.updated_at DESC`, uid)
	if err != nil {
		jsonErr(w, "DB error", 500)
		return
	}
	defer rows.Close()
	trees := []FamilyTree{}
	for rows.Next() {
		var t FamilyTree
		rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt, &t.MemberCount)
		trees = append(trees, t)
	}
	jsonOK(w, trees)
}

func handleCreateTree(w http.ResponseWriter, r *http.Request) {
	uid := userIDFromReq(r)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		jsonErr(w, "Name required", 400)
		return
	}
	var id int64
	err := db.QueryRow(
		`INSERT INTO family_trees(user_id,name,description) VALUES($1,$2,$3) RETURNING id`,
		uid, body.Name, body.Description,
	).Scan(&id)
	if err != nil {
		jsonErr(w, "DB error", 500)
		return
	}
	var t FamilyTree
	db.QueryRow(
		`SELECT id,user_id,name,description,created_at,updated_at FROM family_trees WHERE id=$1`, id,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, t)
}

func handleGetTree(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	var t FamilyTree
	err := db.QueryRow(
		`SELECT id,user_id,name,description,created_at,updated_at FROM family_trees WHERE id=$1 AND user_id=$2`,
		treeID, uid,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		jsonErr(w, "Tree not found", 404)
		return
	}
	db.QueryRow(`SELECT COUNT(*) FROM members WHERE tree_id=$1`, treeID).Scan(&t.MemberCount)
	jsonOK(w, t)
}

func handleUpdateTree(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	db.Exec(
		`UPDATE family_trees SET name=COALESCE(NULLIF($1,name),name), description=$2,
		updated_at=CURRENT_TIMESTAMP WHERE id=$3 AND user_id=$4`,
		body.Name, body.Description, treeID, uid,
	)
	handleGetTree(w, r, treeID)
}

func handleDeleteTree(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	db.Exec(`DELETE FROM family_trees WHERE id=$1 AND user_id=$2`, treeID, uid)
	w.WriteHeader(http.StatusNoContent)
}

// ── Member handlers ───────────────────────────────────────────────────────────

func handleListMembers(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	var ownerID int64
	db.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}
	rows, err := db.Query(
		`SELECT id,tree_id,name,born,died,gender,relation,location,bio,avatar,color,generation,position_x,position_y
		 FROM members WHERE tree_id=$1 ORDER BY generation,id`, treeID)
	if err != nil {
		jsonErr(w, "DB error", 500)
		return
	}
	defer rows.Close()
	members := []Member{}
	for rows.Next() {
		var m Member
		rows.Scan(&m.ID, &m.TreeID, &m.Name, &m.Born, &m.Died, &m.Gender, &m.Relation, &m.Location, &m.Bio, &m.Avatar, &m.Color, &m.Generation, &m.PositionX, &m.PositionY)
		members = append(members, m)
	}
	jsonOK(w, members)
}

func handleAddMember(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	var ownerID int64
	db.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}

	var m Member
	json.NewDecoder(r.Body).Decode(&m)
	if m.Name == "" {
		jsonErr(w, "Name required", 400)
		return
	}
	if m.Avatar == "" {
		words := strings.Fields(m.Name)
		av := ""
		for i, w2 := range words {
			if i >= 2 {
				break
			}
			av += strings.ToUpper(string([]rune(w2)[0]))
		}
		m.Avatar = av
	}
	if m.Color == "" {
		m.Color = "#4A7C59"
	}

	var id int64
	err := db.QueryRow(
		`INSERT INTO members(tree_id,name,born,died,gender,relation,location,bio,avatar,color,generation,position_x,position_y)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		treeID, m.Name, m.Born, m.Died, m.Gender, m.Relation, m.Location, m.Bio, m.Avatar, m.Color, m.Generation, m.PositionX, m.PositionY,
	).Scan(&id)
	if err != nil {
		jsonErr(w, "DB error", 500)
		return
	}
	m.ID = id
	m.TreeID = treeID

	db.Exec(`UPDATE family_trees SET updated_at=CURRENT_TIMESTAMP WHERE id=$1`, treeID)

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, m)
}

func handleUpdateMember(w http.ResponseWriter, r *http.Request, memberID int64) {
	uid := userIDFromReq(r)
	var m Member
	json.NewDecoder(r.Body).Decode(&m)
	var ownerID int64
	db.QueryRow(
		`SELECT t.user_id FROM members mb JOIN family_trees t ON t.id=mb.tree_id WHERE mb.id=$1`, memberID,
	).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}
	db.Exec(
		`UPDATE members SET name=$1,born=$2,died=$3,gender=$4,relation=$5,location=$6,bio=$7,avatar=$8,color=$9,generation=$10,position_x=$11,position_y=$12 WHERE id=$13`,
		m.Name, m.Born, m.Died, m.Gender, m.Relation, m.Location, m.Bio, m.Avatar, m.Color, m.Generation, m.PositionX, m.PositionY, memberID,
	)
	m.ID = memberID
	jsonOK(w, m)
}

func handleDeleteMember(w http.ResponseWriter, r *http.Request, memberID int64) {
	uid := userIDFromReq(r)
	var ownerID int64
	db.QueryRow(
		`SELECT t.user_id FROM members mb JOIN family_trees t ON t.id=mb.tree_id WHERE mb.id=$1`, memberID,
	).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}
	db.Exec(`DELETE FROM members WHERE id=$1`, memberID)
	w.WriteHeader(http.StatusNoContent)
}

// ── Relationship handlers ─────────────────────────────────────────────────────

func handleListRelationships(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	var ownerID int64
	db.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}
	rows, _ := db.Query(
		`SELECT id,tree_id,from_id,to_id,rel_type,label FROM relationships WHERE tree_id=$1`, treeID,
	)
	defer rows.Close()
	rels := []Relationship{}
	for rows.Next() {
		var rel Relationship
		rows.Scan(&rel.ID, &rel.TreeID, &rel.FromID, &rel.ToID, &rel.RelType, &rel.Label)
		rels = append(rels, rel)
	}
	jsonOK(w, rels)
}

func handleAddRelationship(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := userIDFromReq(r)
	var ownerID int64
	db.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}
	var rel Relationship
	json.NewDecoder(r.Body).Decode(&rel)
	rel.TreeID = treeID
	var id int64
	err := db.QueryRow(
		`INSERT INTO relationships(tree_id,from_id,to_id,rel_type,label) VALUES($1,$2,$3,$4,$5)
		 ON CONFLICT (from_id,to_id,rel_type) DO NOTHING RETURNING id`,
		treeID, rel.FromID, rel.ToID, rel.RelType, rel.Label,
	).Scan(&id)
	if err != nil {
		jsonErr(w, "DB error", 500)
		return
	}
	rel.ID = id
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, rel)
}

func handleDeleteRelationship(w http.ResponseWriter, r *http.Request, relID int64) {
	uid := userIDFromReq(r)
	var ownerID int64
	db.QueryRow(
		`SELECT t.user_id FROM relationships rl JOIN family_trees t ON t.id=rl.tree_id WHERE rl.id=$1`, relID,
	).Scan(&ownerID)
	if ownerID != uid {
		jsonErr(w, "Forbidden", 403)
		return
	}
	db.Exec(`DELETE FROM relationships WHERE id=$1`, relID)
	w.WriteHeader(http.StatusNoContent)
}

// ── Router ────────────────────────────────────────────────────────────────────

func route(mux *http.ServeMux) {
	// Auth
	mux.HandleFunc("/api/auth/register", method("POST", handleRegister))
	mux.HandleFunc("/api/auth/login", method("POST", handleLogin))
	mux.HandleFunc("/api/auth/me", method("GET", authMiddleware(handleMe)))
	mux.HandleFunc("/api/auth/me/update", method("PUT", authMiddleware(handleUpdateMe)))

	// Trees
	mux.HandleFunc("/api/trees", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handleListTrees(w, r)
		case "POST":
			handleCreateTree(w, r)
		default:
			jsonErr(w, "Method not allowed", 405)
		}
	}))

	mux.HandleFunc("/api/trees/", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			jsonErr(w, "Not found", 404)
			return
		}
		treeID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			jsonErr(w, "Bad tree ID", 400)
			return
		}

		if len(parts) == 3 {
			switch r.Method {
			case "GET":
				handleGetTree(w, r, treeID)
			case "PUT":
				handleUpdateTree(w, r, treeID)
			case "DELETE":
				handleDeleteTree(w, r, treeID)
			default:
				jsonErr(w, "Method not allowed", 405)
			}
			return
		}

		sub := parts[3]

		if sub == "members" {
			if len(parts) == 4 {
				switch r.Method {
				case "GET":
					handleListMembers(w, r, treeID)
				case "POST":
					handleAddMember(w, r, treeID)
				default:
					jsonErr(w, "Method not allowed", 405)
				}
				return
			}
			memberID, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil {
				jsonErr(w, "Bad member ID", 400)
				return
			}
			switch r.Method {
			case "PUT":
				handleUpdateMember(w, r, memberID)
			case "DELETE":
				handleDeleteMember(w, r, memberID)
			default:
				jsonErr(w, "Method not allowed", 405)
			}
			return
		}

		if sub == "relationships" {
			if len(parts) == 4 {
				switch r.Method {
				case "GET":
					handleListRelationships(w, r, treeID)
				case "POST":
					handleAddRelationship(w, r, treeID)
				default:
					jsonErr(w, "Method not allowed", 405)
				}
				return
			}
			relID, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil {
				jsonErr(w, "Bad rel ID", 400)
				return
			}
			switch r.Method {
			case "DELETE":
				handleDeleteRelationship(w, r, relID)
			default:
				jsonErr(w, "Method not allowed", 405)
			}
			return
		}

		jsonErr(w, "Not found", 404)
	}))

	// Serve the React SPA from ./static/
	mux.Handle("/", http.FileServer(http.Dir("./static")))
}

func method(m string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != m {
			jsonErr(w, "Method not allowed", 405)
			return
		}
		h(w, r)
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	initDB()
	mux := http.NewServeMux()
	route(mux)
	port := envOr("PORT", "8080")
	log.Printf("🌳 Rootswood running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, cors(mux)))
}
