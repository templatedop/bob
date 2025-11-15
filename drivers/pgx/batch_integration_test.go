// +build integration

package pgx

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestBatchIntegration(t *testing.T) {
	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2)),
	)
	if err != nil {
		t.Fatalf("Failed to start postgres container: %v", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(postgresContainer); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}()

	// Get connection string
	dsn, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Create pool
	pool, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Create test table
	_, err = pool.ExecContext(ctx, `
		CREATE TABLE batch_integration_test (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			value INT NOT NULL,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	t.Run("batch insert performance", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback(ctx)

		batch := &pgx.Batch{}

		// Queue 1000 inserts
		for i := 0; i < 1000; i++ {
			batch.Queue(
				"INSERT INTO batch_integration_test (name, value) VALUES ($1, $2)",
				fmt.Sprintf("item_%d", i),
				i,
			)
		}

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// Process all results
		for i := 0; i < 1000; i++ {
			tag, err := results.Exec()
			if err != nil {
				t.Fatalf("Failed to execute batch statement %d: %v", i, err)
			}
			if tag.RowsAffected() != 1 {
				t.Errorf("Expected 1 row affected for statement %d, got %d", i, tag.RowsAffected())
			}
		}

		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("Failed to commit transaction: %v", err)
		}

		// Verify count
		rows, err := pool.QueryContext(ctx, "SELECT COUNT(*) FROM batch_integration_test")
		if err != nil {
			t.Fatalf("Failed to query count: %v", err)
		}
		defer rows.Close()

		var count int
		if rows.Next() {
			if err := rows.Scan(&count); err != nil {
				t.Fatalf("Failed to scan count: %v", err)
			}
		}

		if count != 1000 {
			t.Errorf("Expected 1000 rows, got %d", count)
		}

		t.Logf("Successfully inserted and verified %d rows using batch operations", count)
	})

	t.Run("batch with queries and updates", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback(ctx)

		batch := &pgx.Batch{}

		// Mix of operations
		batch.Queue("SELECT COUNT(*) FROM batch_integration_test WHERE value < 100")
		batch.Queue("UPDATE batch_integration_test SET value = value * 2 WHERE value < 10")
		batch.Queue("SELECT MAX(value) FROM batch_integration_test")
		batch.Queue("DELETE FROM batch_integration_test WHERE value > 1500")

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// Query 1: Count
		var count int
		if err := results.QueryRow().Scan(&count); err != nil {
			t.Fatalf("Failed to scan count: %v", err)
		}
		t.Logf("Count of rows with value < 100: %d", count)

		// Update
		tag, err := results.Exec()
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		t.Logf("Updated %d rows", tag.RowsAffected())

		// Query 2: Max value
		var maxValue int
		if err := results.QueryRow().Scan(&maxValue); err != nil {
			t.Fatalf("Failed to scan max value: %v", err)
		}
		t.Logf("Max value: %d", maxValue)

		// Delete
		tag, err = results.Exec()
		if err != nil {
			t.Fatalf("Failed to execute delete: %v", err)
		}
		t.Logf("Deleted %d rows", tag.RowsAffected())

		if err := tx.Commit(ctx); err != nil {
			t.Fatalf("Failed to commit transaction: %v", err)
		}
	})

	t.Run("batch error recovery", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v", err)
		}
		defer tx.Rollback(ctx)

		batch := &pgx.Batch{}

		// Valid insert
		batch.Queue("INSERT INTO batch_integration_test (name, value) VALUES ($1, $2)", "valid", 999)
		// Invalid insert (wrong type)
		batch.Queue("INSERT INTO batch_integration_test (name, value) VALUES ($1, $2)", "invalid", "not_a_number")
		// This should not execute
		batch.Queue("INSERT INTO batch_integration_test (name, value) VALUES ($1, $2)", "should_not_insert", 888)

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// First should succeed
		tag, err := results.Exec()
		if err != nil {
			t.Fatalf("First insert should succeed: %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("Expected 1 row affected, got %d", tag.RowsAffected())
		}

		// Second should fail
		_, err = results.Exec()
		if err == nil {
			t.Error("Expected error for invalid insert, got nil")
		} else {
			t.Logf("Got expected error: %v", err)
		}

		// Transaction should rollback
		if err := tx.Rollback(ctx); err != nil {
			t.Logf("Rollback completed: %v", err)
		}
	})
}
