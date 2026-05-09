package router

import (
	"net/http"
	"strconv"
	"strings"

	"rootswood/handlers"
	"rootswood/middleware"
	"rootswood/utils"
)

func Register(mux *http.ServeMux) {
	// Auth
	mux.HandleFunc("/api/auth/register", method("POST", handlers.HandleRegister))
	mux.HandleFunc("/api/auth/login", method("POST", handlers.HandleLogin))
	mux.HandleFunc("/api/auth/me", method("GET", middleware.AuthMiddleware(handlers.HandleMe)))
	mux.HandleFunc("/api/auth/me/update", method("PUT", middleware.AuthMiddleware(handlers.HandleUpdateMe)))

	// Trees
	mux.HandleFunc("/api/trees", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			handlers.HandleListTrees(w, r)
		case "POST":
			handlers.HandleCreateTree(w, r)
		default:
			utils.JsonErr(w, "Method not allowed", 405)
		}
	}))

	mux.HandleFunc("/api/trees/", middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 {
			utils.JsonErr(w, "Not found", 404)
			return
		}
		treeID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			utils.JsonErr(w, "Bad tree ID", 400)
			return
		}

		if len(parts) == 3 {
			switch r.Method {
			case "GET":
				handlers.HandleGetTree(w, r, treeID)
			case "PUT":
				handlers.HandleUpdateTree(w, r, treeID)
			case "DELETE":
				handlers.HandleDeleteTree(w, r, treeID)
			default:
				utils.JsonErr(w, "Method not allowed", 405)
			}
			return
		}

		sub := parts[3]

		if sub == "members" {
			if len(parts) == 4 {
				switch r.Method {
				case "GET":
					handlers.HandleListMembers(w, r, treeID)
				case "POST":
					handlers.HandleAddMember(w, r, treeID)
				default:
					utils.JsonErr(w, "Method not allowed", 405)
				}
				return
			}
			memberID, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil {
				utils.JsonErr(w, "Bad member ID", 400)
				return
			}
			switch r.Method {
			case "PUT":
				handlers.HandleUpdateMember(w, r, memberID)
			case "DELETE":
				handlers.HandleDeleteMember(w, r, memberID)
			default:
				utils.JsonErr(w, "Method not allowed", 405)
			}
			return
		}

		if sub == "relationships" {
			if len(parts) == 4 {
				switch r.Method {
				case "GET":
					handlers.HandleListRelationships(w, r, treeID)
				case "POST":
					handlers.HandleAddRelationship(w, r, treeID)
				default:
					utils.JsonErr(w, "Method not allowed", 405)
				}
				return
			}
			relID, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil {
				utils.JsonErr(w, "Bad rel ID", 400)
				return
			}
			switch r.Method {
			case "DELETE":
				handlers.HandleDeleteRelationship(w, r, relID)
			default:
				utils.JsonErr(w, "Method not allowed", 405)
			}
			return
		}

		utils.JsonErr(w, "Not found", 404)
	}))

	// Serve the React SPA from ./static/
	mux.Handle("/", http.FileServer(http.Dir("./static")))
}

func method(m string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != m {
			utils.JsonErr(w, "Method not allowed", 405)
			return
		}
		h(w, r)
	}
}
