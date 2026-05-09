package config

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"rootswood/models"

	jwt "github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
)

var JwtSecret = []byte(EnvOr("JWT_SECRET", "rootswood-secret-change-in-prod"))
var DB *sql.DB

func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func InitDB() {
	var err error
	DB, err = sql.Open("postgres", EnvOr("DATABASE_URL", "postgres://localhost/rootswood?sslmode=disable"))
	if err != nil {
		log.Fatal("DB open:", err)
	}
	if err = DB.Ping(); err != nil {
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

	if _, err = DB.Exec(schema); err != nil {
		log.Fatal("Schema:", err)
	}
	log.Println("✅ Database ready")
}

// ── JWT helpers ───────────────────────────────────────────────────────────────

func SignToken(u models.User) (string, error) {
	claims := models.Claims{
		UserID: u.ID,
		Email:  u.Email,
		Name:   u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JwtSecret)
}

func ParseBearerToken(r *http.Request) (*models.Claims, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	token, err := jwt.ParseWithClaims(tokenStr, &models.Claims{}, func(t *jwt.Token) (interface{}, error) {
		return JwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return token.Claims.(*models.Claims), nil
}
