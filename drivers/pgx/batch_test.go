package pgx

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestBatchOperations(t *testing.T) {
	// Skip if no database connection is available
	dsn := os.Getenv("PGX_TEST_DSN")
	if dsn == "" {
		t.Skip("PGX_TEST_DSN not set, skipping batch operations test")
	}

	ctx := context.Background()

	// Create a connection pool
	pool, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Create a test table
	_, err = pool.ExecContext(ctx, `
		CREATE TEMPORARY TABLE batch_test (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			value INT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Start a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	t.Run("SendBatch with multiple inserts", func(t *testing.T) {
		// Create a batch
		batch := &pgx.Batch{}

		// Queue multiple insert statements
		batch.Queue("INSERT INTO batch_test (name, value) VALUES ($1, $2)", "item1", 100)
		batch.Queue("INSERT INTO batch_test (name, value) VALUES ($1, $2)", "item2", 200)
		batch.Queue("INSERT INTO batch_test (name, value) VALUES ($1, $2)", "item3", 300)

		// Send the batch
		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// Check each result
		for i := 0; i < 3; i++ {
			tag, err := results.Exec()
			if err != nil {
				t.Fatalf("Failed to execute batch statement %d: %v", i, err)
			}
			if tag.RowsAffected() != 1 {
				t.Errorf("Expected 1 row affected for statement %d, got %d", i, tag.RowsAffected())
			}
		}
	})

	t.Run("SendBatch with mixed operations", func(t *testing.T) {
		batch := &pgx.Batch{}

		// Insert
		batch.Queue("INSERT INTO batch_test (name, value) VALUES ($1, $2)", "batch_item", 400)
		// Query
		batch.Queue("SELECT COUNT(*) FROM batch_test WHERE value > $1", 150)
		// Update
		batch.Queue("UPDATE batch_test SET value = value + 10 WHERE name = $1", "item1")

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		// Check insert
		tag, err := results.Exec()
		if err != nil {
			t.Fatalf("Failed to execute insert: %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("Expected 1 row affected for insert, got %d", tag.RowsAffected())
		}

		// Check query
		var count int
		err = results.QueryRow().Scan(&count)
		if err != nil {
			t.Fatalf("Failed to scan query result: %v", err)
		}
		if count < 2 {
			t.Errorf("Expected at least 2 rows with value > 150, got %d", count)
		}

		// Check update
		tag, err = results.Exec()
		if err != nil {
			t.Fatalf("Failed to execute update: %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("Expected 1 row affected for update, got %d", tag.RowsAffected())
		}
	})

	t.Run("SendBatch with PoolConn", func(t *testing.T) {
		// Test with a pool connection
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Failed to acquire connection: %v", err)
		}
		defer conn.Release()

		connTx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatalf("Failed to begin transaction on connection: %v", err)
		}
		defer connTx.Rollback(ctx)

		batch := &pgx.Batch{}
		batch.Queue("INSERT INTO batch_test (name, value) VALUES ($1, $2)", "conn_item", 500)

		results := connTx.SendBatch(ctx, batch)
		defer results.Close()

		tag, err := results.Exec()
		if err != nil {
			t.Fatalf("Failed to execute batch on connection: %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Errorf("Expected 1 row affected, got %d", tag.RowsAffected())
		}
	})

	t.Run("SendBatch error handling", func(t *testing.T) {
		batch := &pgx.Batch{}

		// Queue a statement that will fail
		batch.Queue("INSERT INTO batch_test (name, value) VALUES ($1, $2)", "error_item", "not_a_number")

		results := tx.SendBatch(ctx, batch)
		defer results.Close()

		_, err := results.Exec()
		if err == nil {
			t.Error("Expected error for invalid value type, got nil")
		}
	})

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify data was committed
	var finalCount int
	rows, err := pool.QueryContext(ctx, "SELECT COUNT(*) FROM batch_test")
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&finalCount); err != nil {
			t.Fatalf("Failed to scan count: %v", err)
		}
		t.Logf("Total rows in batch_test: %d", finalCount)
	}
}

func TestBatchWithStandaloneConn(t *testing.T) {
	dsn := os.Getenv("PGX_TEST_DSN")
	if dsn == "" {
		t.Skip("PGX_TEST_DSN not set, skipping standalone conn batch test")
	}

	ctx := context.Background()

	// Create a standalone connection
	pgxConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer pgxConn.Close(ctx)

	conn := NewConn(pgxConn)

	// Create test table
	_, err = conn.ExecContext(ctx, `
		CREATE TEMPORARY TABLE batch_conn_test (
			id SERIAL PRIMARY KEY,
			data TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	batch.Queue("INSERT INTO batch_conn_test (data) VALUES ($1)", "test1")
	batch.Queue("INSERT INTO batch_conn_test (data) VALUES ($1)", "test2")

	results := tx.SendBatch(ctx, batch)
	defer results.Close()

	for i := 0; i < 2; i++ {
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
}
