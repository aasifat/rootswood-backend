package handlers

import (
	"encoding/json"
	"net/http"

	"rootswood/config"
	"rootswood/models"
	"rootswood/utils"
)

func HandleListTrees(w http.ResponseWriter, r *http.Request) {
	uid := utils.UserIDFromReq(r)
	rows, err := config.DB.Query(`
		SELECT t.id,t.user_id,t.name,t.description,t.created_at,t.updated_at,
		       COUNT(m.id) as member_count
		FROM family_trees t
		LEFT JOIN members m ON m.tree_id=t.id
		WHERE t.user_id=$1
		GROUP BY t.id
		ORDER BY t.updated_at DESC`, uid)
	if err != nil {
		utils.JsonErr(w, "DB error", 500)
		return
	}
	defer rows.Close()
	trees := []models.FamilyTree{}
	for rows.Next() {
		var t models.FamilyTree
		rows.Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt, &t.MemberCount)
		trees = append(trees, t)
	}
	utils.JsonOK(w, trees)
}

func HandleCreateTree(w http.ResponseWriter, r *http.Request) {
	uid := utils.UserIDFromReq(r)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		utils.JsonErr(w, "Name required", 400)
		return
	}
	var id int64
	err := config.DB.QueryRow(
		`INSERT INTO family_trees(user_id,name,description) VALUES($1,$2,$3) RETURNING id`,
		uid, body.Name, body.Description,
	).Scan(&id)
	if err != nil {
		utils.JsonErr(w, "DB error", 500)
		return
	}
	var t models.FamilyTree
	config.DB.QueryRow(
		`SELECT id,user_id,name,description,created_at,updated_at FROM family_trees WHERE id=$1`, id,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	w.WriteHeader(http.StatusCreated)
	utils.JsonOK(w, t)
}

func HandleGetTree(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	var t models.FamilyTree
	err := config.DB.QueryRow(
		`SELECT id,user_id,name,description,created_at,updated_at FROM family_trees WHERE id=$1 AND user_id=$2`,
		treeID, uid,
	).Scan(&t.ID, &t.UserID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		utils.JsonErr(w, "Tree not found", 404)
		return
	}
	config.DB.QueryRow(`SELECT COUNT(*) FROM members WHERE tree_id=$1`, treeID).Scan(&t.MemberCount)
	utils.JsonOK(w, t)
}

func HandleUpdateTree(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	config.DB.Exec(
		`UPDATE family_trees SET name=COALESCE(NULLIF($1,name),name), description=$2,
		updated_at=CURRENT_TIMESTAMP WHERE id=$3 AND user_id=$4`,
		body.Name, body.Description, treeID, uid,
	)
	HandleGetTree(w, r, treeID)
}

func HandleDeleteTree(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	config.DB.Exec(`DELETE FROM family_trees WHERE id=$1 AND user_id=$2`, treeID, uid)
	w.WriteHeader(http.StatusNoContent)
}
