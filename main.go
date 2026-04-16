package main

import (
	"log"
	"net/http"
	"os"

	"profiles-api/db"
	"profiles-api/handlers"
)

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		next(w, r)
	}
}

func main() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/profiles_db"
	}

	err := db.Connect(connStr)
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/profiles", corsMiddleware(handlers.CreateProfile))
	mux.HandleFunc("GET /api/profiles/{id}", corsMiddleware(handlers.GetProfileById))
	mux.HandleFunc("GET /api/profiles", corsMiddleware(handlers.GetProfiles))
	mux.HandleFunc("DELETE /api/profiles/{id}", corsMiddleware(handlers.DeleteProfile))

	port := os.Getenv("PORT")
	if port == "" {
		log.Printf("Couldn't reach port :%s", port)
	}

	srv := http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Server running on :%s", port)
	log.Fatal(srv.ListenAndServe())
}
