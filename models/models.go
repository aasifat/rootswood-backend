package models

import (
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

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
