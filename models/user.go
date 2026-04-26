package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        uuid.UUID  `json:"id"`
	GithubId  string     `json:"github_id"`
	AvatarUrl *string    `json:"avatar_url"`
	UserName  string     `json:"username"`
	Email     *string    `json:"email"`
	Role      string     `json:"role"`
	IsActive  bool       `json:"is_active"`
	LastLogin *time.Time `json:"last_login_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type GithubUser struct {
	ID        int64   `json:"id"`
	Login     string  `json:"login"`
	Email     *string `json:"email"`
	AvatarURL *string `json:"avatar_url"`
}

type MyClaim struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}
