package handlers

import (
	"encoding/json"
	"net/http"

	"rootswood/config"
	"rootswood/models"
	"rootswood/utils"
)

func HandleListRelationships(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	var ownerID int64
	config.DB.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}
	rows, _ := config.DB.Query(
		`SELECT id,tree_id,from_id,to_id,rel_type,label FROM relationships WHERE tree_id=$1`, treeID,
	)
	defer rows.Close()
	rels := []models.Relationship{}
	for rows.Next() {
		var rel models.Relationship
		rows.Scan(&rel.ID, &rel.TreeID, &rel.FromID, &rel.ToID, &rel.RelType, &rel.Label)
		rels = append(rels, rel)
	}
	utils.JsonOK(w, rels)
}

func HandleAddRelationship(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	var ownerID int64
	config.DB.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}
	var rel models.Relationship
	json.NewDecoder(r.Body).Decode(&rel)
	rel.TreeID = treeID
	var id int64
	err := config.DB.QueryRow(
		`INSERT INTO relationships(tree_id,from_id,to_id,rel_type,label) VALUES($1,$2,$3,$4,$5)
		 ON CONFLICT (from_id,to_id,rel_type) DO NOTHING RETURNING id`,
		treeID, rel.FromID, rel.ToID, rel.RelType, rel.Label,
	).Scan(&id)
	if err != nil {
		utils.JsonErr(w, "DB error", 500)
		return
	}
	rel.ID = id
	w.WriteHeader(http.StatusCreated)
	utils.JsonOK(w, rel)
}

func HandleDeleteRelationship(w http.ResponseWriter, r *http.Request, relID int64) {
	uid := utils.UserIDFromReq(r)
	var ownerID int64
	config.DB.QueryRow(
		`SELECT t.user_id FROM relationships rl JOIN family_trees t ON t.id=rl.tree_id WHERE rl.id=$1`, relID,
	).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}
	config.DB.Exec(`DELETE FROM relationships WHERE id=$1`, relID)
	w.WriteHeader(http.StatusNoContent)
}
