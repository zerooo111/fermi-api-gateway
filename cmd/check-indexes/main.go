package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fmt.Fprintf(os.Stderr, "ERROR: DATABASE_URL not set\n")
		os.Exit(1)
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	query := `
		SELECT 
			indexname,
			indexdef
		FROM pg_indexes
		WHERE tablename = 'market_prices'
		  AND indexname LIKE 'idx_market_prices%'
		ORDER BY indexname;
	`

	rows, err := db.Query(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	fmt.Println("Indexes on market_prices table:")
	fmt.Println("-----------------------------------")
	count := 0
	for rows.Next() {
		var indexName, indexDef string
		if err := rows.Scan(&indexName, &indexDef); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Scan failed: %v\n", err)
			continue
		}
		fmt.Printf("\n%s:\n%s\n", indexName, indexDef)
		count++
	}
	if count == 0 {
		fmt.Println("No indexes found (table might not exist yet)")
	} else {
		fmt.Printf("\nTotal indexes: %d\n", count)
	}
}

