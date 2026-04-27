package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"profiles-api/models"

	"github.com/golang-jwt/jwt/v5"
)

type UserContext struct {
	ID   string
	Role string
}

type contextKey string

const (
	UserCtxKey contextKey = "user_data"
)

func ValidateToken(tokenString string) (*models.MyClaim, error) {
	mySecret := []byte(os.Getenv("JWT_SECRET"))

	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &models.MyClaim{}, func(t *jwt.Token) (any, error) {
		// Ensure the signing method is what we expect
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return mySecret, nil
	})

	// Check if token is valid and extract claims
	if claims, ok := token.Claims.(*models.MyClaim); ok && token.Valid {
		return claims, nil
	} else {
		return nil, err
	}
}

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		h := r.Header.Get("Authorization")
		if h == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "error",
				"message": "Your aren't authorized",
			})
			return
		}

		tokenString := strings.TrimPrefix(h, "Bearer ")
		claims, err := ValidateToken(tokenString)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		UserData := UserContext{
			ID:   claims.ID,
			Role: claims.Role,
		}

		// putting a value in (middleware)
		ctx := context.WithValue(r.Context(), UserCtxKey, UserData)
		r = r.WithContext(ctx)
		next(w, r)
	}
}

func RBACMiddleware(requiredRole string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 1. Retrieve claims from context (populated by AuthMiddleware)
			userData, ok := r.Context().Value(UserCtxKey).(UserContext)
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "Identity not found"})
				return
			}

			// 2. Check if the user has the required role
			// Or check if they are an 'admin' (who usually bypasses all checks)
			if userData.Role != requiredRole && userData.Role != "admin" {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"error": fmt.Sprintf("Access denied: %s role required", requiredRole),
				})
				return
			}

			// 3. Role matches! Proceed.
			next(w, r)
		}
	}
}

func VersionMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val := r.Header.Get("X-API-Version")
		if val == "" || val != "1" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "API version header required",
			})
			return
		}

		next(w, r)
	}
}
