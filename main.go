package main

// ─────────────────────────────────────────────────────────────────────────────
//  Rootswood Backend  ·  Go 1.22+
//  Modular server with JWT auth, PostgreSQL persistence, full CRUD.
//
//  Dependencies (go get):
//    github.com/golang-jwt/jwt/v5
//    github.com/lib/pq
//    golang.org/x/crypto
//    github.com/joho/godotenv
//
//  Run:
//    DATABASE_URL=postgres://... go run main.go
//
//  API base: http://localhost:8080/api
// ─────────────────────────────────────────────────────────────────────────────

import (
	"log"
	"net/http"

	"rootswood/config"
	"rootswood/middleware"
	"rootswood/router"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load() // Load .env file if present (for local development)
	config.InitDB()

	mux := http.NewServeMux()
	router.Register(mux)

	port := config.EnvOr("PORT", "8080")
	log.Printf("🌳 Rootswood running on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, middleware.Cors(mux)))
}
