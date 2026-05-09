package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"rootswood/config"
	"rootswood/models"
	"rootswood/utils"
)

func HandleListMembers(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	var ownerID int64
	config.DB.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}
	rows, err := config.DB.Query(
		`SELECT id,tree_id,name,born,died,gender,relation,location,bio,avatar,color,generation,position_x,position_y
		 FROM members WHERE tree_id=$1 ORDER BY generation,id`, treeID)
	if err != nil {
		utils.JsonErr(w, "DB error", 500)
		return
	}
	defer rows.Close()
	members := []models.Member{}
	for rows.Next() {
		var m models.Member
		rows.Scan(&m.ID, &m.TreeID, &m.Name, &m.Born, &m.Died, &m.Gender, &m.Relation, &m.Location, &m.Bio, &m.Avatar, &m.Color, &m.Generation, &m.PositionX, &m.PositionY)
		members = append(members, m)
	}
	utils.JsonOK(w, members)
}

func HandleAddMember(w http.ResponseWriter, r *http.Request, treeID int64) {
	uid := utils.UserIDFromReq(r)
	var ownerID int64
	config.DB.QueryRow(`SELECT user_id FROM family_trees WHERE id=$1`, treeID).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}

	var m models.Member
	json.NewDecoder(r.Body).Decode(&m)
	if m.Name == "" {
		utils.JsonErr(w, "Name required", 400)
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
	err := config.DB.QueryRow(
		`INSERT INTO members(tree_id,name,born,died,gender,relation,location,bio,avatar,color,generation,position_x,position_y)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`,
		treeID, m.Name, m.Born, m.Died, m.Gender, m.Relation, m.Location, m.Bio, m.Avatar, m.Color, m.Generation, m.PositionX, m.PositionY,
	).Scan(&id)
	if err != nil {
		utils.JsonErr(w, "DB error", 500)
		return
	}
	m.ID = id
	m.TreeID = treeID

	config.DB.Exec(`UPDATE family_trees SET updated_at=CURRENT_TIMESTAMP WHERE id=$1`, treeID)

	w.WriteHeader(http.StatusCreated)
	utils.JsonOK(w, m)
}

func HandleUpdateMember(w http.ResponseWriter, r *http.Request, memberID int64) {
	uid := utils.UserIDFromReq(r)
	var m models.Member
	json.NewDecoder(r.Body).Decode(&m)
	var ownerID int64
	config.DB.QueryRow(
		`SELECT t.user_id FROM members mb JOIN family_trees t ON t.id=mb.tree_id WHERE mb.id=$1`, memberID,
	).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}
	config.DB.Exec(
		`UPDATE members SET name=$1,born=$2,died=$3,gender=$4,relation=$5,location=$6,bio=$7,avatar=$8,color=$9,generation=$10,position_x=$11,position_y=$12 WHERE id=$13`,
		m.Name, m.Born, m.Died, m.Gender, m.Relation, m.Location, m.Bio, m.Avatar, m.Color, m.Generation, m.PositionX, m.PositionY, memberID,
	)
	m.ID = memberID
	utils.JsonOK(w, m)
}

func HandleDeleteMember(w http.ResponseWriter, r *http.Request, memberID int64) {
	uid := utils.UserIDFromReq(r)
	var ownerID int64
	config.DB.QueryRow(
		`SELECT t.user_id FROM members mb JOIN family_trees t ON t.id=mb.tree_id WHERE mb.id=$1`, memberID,
	).Scan(&ownerID)
	if ownerID != uid {
		utils.JsonErr(w, "Forbidden", 403)
		return
	}
	config.DB.Exec(`DELETE FROM members WHERE id=$1`, memberID)
	w.WriteHeader(http.StatusNoContent)
}
