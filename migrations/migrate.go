// Package main provides database migration utility for BinCrypt PostgreSQL backend.
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var migrations embed.FS

func main() {
	dbURL := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	direction := flag.String("direction", "up", "Migration direction: up or down")
	flag.Parse()

	if *dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, *dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := ensureMigrationsTable(ctx, pool); err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	if *direction == "up" {
		if err := migrateUp(ctx, pool); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations applied successfully")
	} else {
		log.Println("Rollback not implemented - use manual SQL for rollback")
	}
}

func ensureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func migrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := migrations.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read migrations: %w", err)
	}

	var sqlFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			sqlFiles = append(sqlFiles, f.Name())
		}
	}
	sort.Strings(sqlFiles)

	for _, file := range sqlFiles {
		version := strings.TrimSuffix(file, filepath.Ext(file))

		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		if exists {
			log.Printf("Skipping migration %s (already applied)", file)
			continue
		}

		content, err := migrations.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}

		log.Printf("Applying migration %s", file)

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to apply migration %s: %w", file, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		log.Printf("Successfully applied migration %s", file)
	}

	return nil
}
