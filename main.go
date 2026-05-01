package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"profiles-api/db"
	"profiles-api/handlers"
	"profiles-api/middleware" // Assuming your middlewares are here

	"github.com/joho/godotenv"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Version")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if r.Header.Get("Content-Type") == "" ||
			!strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Content-Type", "application/json")
		}

		next(w, r)
	}
}

func main() {
	godotenv.Load()
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://jeremiah:yourpassword@localhost:5432/profiles_db"
	}

	err := db.Connect(connStr)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	err = db.InitializeSchema()
	if err != nil {
		log.Fatalf("Could not initialize db: %v", err)
	}

	mux := http.NewServeMux()

	// --- Auth Routes (rate limited to 10/min) ---
	mux.HandleFunc("GET /auth/github",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(10, time.Minute)(
				corsMiddleware(handlers.GithubLogin))))

	mux.HandleFunc("GET /auth/github/callback",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(10, time.Minute)(
				corsMiddleware(handlers.GithubCallback))))

	mux.HandleFunc("POST /auth/refresh",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(10, time.Minute)(
				corsMiddleware(handlers.RefreshToken))))

	mux.HandleFunc("POST /auth/logout",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(10, time.Minute)(
				corsMiddleware(handlers.Logout))))

	// --- Protected Profile Routes (rate limited to 60/min) ---
	mux.HandleFunc("POST /api/profiles",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(
					middleware.RBACMiddleware("admin")(handlers.CreateProfile)))))))

	mux.HandleFunc("GET /api/profiles/search",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.SearchProfiles))))))

	mux.HandleFunc("GET /api/profiles/export",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.ExportProfiles))))))

	mux.HandleFunc("GET /api/profiles/{id}",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.GetProfileById))))))

	mux.HandleFunc("GET /api/profiles",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.GetProfiles))))))

	mux.HandleFunc("DELETE /api/profiles/{id}",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(
					middleware.RBACMiddleware("admin")(handlers.DeleteProfile)))))))

	mux.HandleFunc("GET /api/users/me",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(60, time.Minute)(
				corsMiddleware(middleware.VersionMiddleware(
					middleware.AuthMiddleware(handlers.GetMe))))))

	// --- CLI Callback
	mux.HandleFunc("POST /auth/github/callback",
		middleware.LoggingMiddleware(
			middleware.RateLimiterMiddleWare(19, time.Minute)(
				corsMiddleware(handlers.GithubCallbackCLI))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Server running on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
