package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Get DATABASE_URL from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: DATABASE_URL environment variable not set\n")
		os.Exit(1)
	}

	// Get migration file from command line args
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <migration_file.sql>\n", os.Args[0])
		os.Exit(1)
	}
	migrationFile := os.Args[1]

	// Read migration file
	sqlBytes, err := ioutil.ReadFile(migrationFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to read migration file %s: %v\n", migrationFile, err)
		os.Exit(1)
	}

	// Connect to database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	// Execute migration
	fmt.Printf("Running migration: %s\n", migrationFile)
	if _, err := db.Exec(string(sqlBytes)); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migration completed successfully!")
}

