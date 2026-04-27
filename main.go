package main

import (
	"log"
	"net/http"
	"os"

	"profiles-api/db"
	"profiles-api/handlers"
	"profiles-api/middleware" // Assuming your middlewares are here

	"github.com/joho/godotenv"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// For preflight requests
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			return
		}

		next(w, r)
	}
}

func main() {
	godotenv.Load()
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://jeremiah:newpassword@localhost:5432/profiles_db"
	}

	err := db.Connect(connStr)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	err = db.InitializeSchema()
	if err != nil {
		log.Fatalf("Could initialized db: %v", err)
	}

	mux := http.NewServeMux()

	// --- Public Auth Routes ---
	mux.HandleFunc("GET /auth/github", corsMiddleware(handlers.GithubLogin))
	mux.HandleFunc("GET /auth/github/callback", corsMiddleware(handlers.GithubCallback))
	mux.HandleFunc("POST /auth/refresh", corsMiddleware(handlers.RefreshToken))
	mux.HandleFunc("POST /auth/logout", corsMiddleware(handlers.Logout))

	// --- Protected Profile Routes ---
	// Wrapped in AuthMiddleware so only logged-in users can access
	mux.HandleFunc("POST /api/profiles",
		corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(
			middleware.RBACMiddleware("admin")(handlers.CreateProfile),
		))))

	mux.HandleFunc("GET /api/profiles/search",
		corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.SearchProfiles))))

	mux.HandleFunc("GET /api/profiles/export",
		corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.GetProfileById))))

	mux.HandleFunc("GET /api/profiles/{id}",
		corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.GetProfileById))))

	mux.HandleFunc("GET /api/profiles",
		corsMiddleware(
			middleware.VersionMiddleware(middleware.AuthMiddleware(handlers.GetProfiles)),
		))

	// Example of nested Middleware: Only Admins can delete profiles
	mux.HandleFunc("DELETE /api/profiles/{id}",
		corsMiddleware(middleware.VersionMiddleware(middleware.AuthMiddleware(
			middleware.RBACMiddleware("admin")(handlers.DeleteProfile),
		))))

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
