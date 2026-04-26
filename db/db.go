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

func InitializeSchema() error {
	const tableSchema = `
    CREATE TABLE IF NOT EXISTS profiles (
        id UUID PRIMARY KEY,
        name VARCHAR NOT NULL UNIQUE,
        gender VARCHAR CHECK (gender IN ('male', 'female')),
        gender_probability FLOAT,
        age INT,
        age_group VARCHAR CHECK (age_group IN ('child', 'teenager', 'adult', 'senior')),
        country_id VARCHAR(2),
        country_name VARCHAR,
        country_probability FLOAT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`

	const userSchema = `
    CREATE TABLE IF NOT EXISTS users (
    	id UUID PRIMARY KEY,
		github_id VARCHAR UNIQUE,
		avatar_url TEXT,
		username TEXT,
		email TEXT UNIQUE,
		role VARCHAR CHECK (role IN ('admin', 'analyst')) DEFAULT 'analyst',
		is_active BOOL DEFAULT TRUE,
		last_login_at TIMESTAMPTZ,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP

    );`

	const tokenSchema = `
    CREATE TABLE IF NOT EXISTS tokens (
        id UUID PRIMARY KEY,
		user_id UUID REFERENCES users(id),
		token TEXT,
		expires_at TIMESTAMPTZ,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`

	_, err := DB.Exec(tableSchema)
	if err != nil {
		return err
	}

	_, err = DB.Exec(userSchema)
	if err != nil {
		return err
	}

	_, err = DB.Exec(tokenSchema)
	if err != nil {
		return err
	}

	return nil
}
