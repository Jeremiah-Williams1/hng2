package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DB *sql.DB // This is the global variable other packages will access

func Connect(connStr string) error {
	var err error

	// Assign the connection to the GLOBAL DB variable, not a local one
	DB, err = sql.Open("pgx", connStr)
	if err != nil {
		return err
	}

	// Check if the connection is actually alive
	err = DB.Ping()
	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}
