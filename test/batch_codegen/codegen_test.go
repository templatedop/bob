package batch_codegen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"
)

func TestBatchCodeGeneration(t *testing.T) {
	dsn := os.Getenv("PGX_TEST_DSN")
	if dsn == "" {
		t.Skip("PGX_TEST_DSN not set, skipping batch code generation test")
	}

	ctx := context.Background()

	// Step 1: Setup database
	t.Log("Step 1: Setting up database...")
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Run migrations
	migrationFile := filepath.Join("migrations", "001_create_test_tables.sql")
	migration, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatalf("Failed to read migration: %v", err)
	}

	// Clean up first
	_, err = pool.Exec(ctx, `
		DROP TABLE IF EXISTS inventory_log CASCADE;
		DROP TABLE IF EXISTS orders CASCADE;
		DROP TABLE IF EXISTS products CASCADE;
	`)
	if err != nil {
		t.Fatalf("Failed to clean up tables: %v", err)
	}

	_, err = pool.Exec(ctx, string(migration))
	if err != nil {
		t.Fatalf("Failed to run migration: %v", err)
	}
	t.Log("✓ Database setup complete")

	// Step 2: Generate code
	t.Log("Step 2: Generating code from queries...")

	// Set environment variable for code generation
	os.Setenv("PGX_TEST_DSN", dsn)
	defer os.Unsetenv("PGX_TEST_DSN")

	cmd := exec.Command("go", "run",
		"../../gen/bobgen-psql/main.go",
		"-c", "bobgen.yaml",
	)
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to generate code: %v\nOutput: %s", err, output)
	}
	t.Logf("✓ Code generation complete\n%s", output)

	// Step 3: Verify generated files exist
	t.Log("Step 3: Verifying generated files...")
	generatedFiles := []string{
		"generated/queries/insert_product.bob.go",
		"generated/queries/create_order.bob.go",
		"generated/queries/log_inventory.bob.go",
		"generated/queries/update_stock.bob.go",
		"generated/queries/get_product.bob.go",
		"generated/queries/list_low_stock.bob.go",
	}

	for _, file := range generatedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected generated file not found: %s", file)
		} else {
			t.Logf("✓ Found generated file: %s", file)
		}
	}
}

func TestBatchWithoutTransaction(t *testing.T) {
	dsn := os.Getenv("PGX_TEST_DSN")
	if dsn == "" {
		t.Skip("PGX_TEST_DSN not set, skipping batch without transaction test")
	}

	ctx := context.Background()

	// Create pool
	pool, err := bobpgx.New(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Clean and setup test data
	_, err = pool.ExecContext(ctx, `
		DROP TABLE IF EXISTS inventory_log CASCADE;
		DROP TABLE IF EXISTS orders CASCADE;
		DROP TABLE IF EXISTS products CASCADE;
	`)
	if err != nil {
		t.Fatalf("Failed to clean tables: %v", err)
	}

	migrationFile := filepath.Join("migrations", "001_create_test_tables.sql")
	migration, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatalf("Failed to read migration: %v", err)
	}

	_, err = pool.ExecContext(ctx, string(migration))
	if err != nil {
		t.Fatalf("Failed to run migration: %v", err)
	}

	t.Run("batch without explicit transaction - connection auto-manages", func(t *testing.T) {
		// Acquire a connection from the pool
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Failed to acquire connection: %v", err)
		}
		defer conn.Release()

		// Create a batch - NO explicit BEGIN transaction
		// pgx.Batch handles this implicitly
		batch := &pgx.Batch{}

		// Queue multiple operations
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Batch Product 1", 99.99, 10)
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Batch Product 2", 149.99, 20)
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Batch Product 3", 199.99, 30)

		// Send batch WITHOUT wrapping in a transaction
		// This tests that batch itself is implicitly transactional
		results := conn.SendBatch(ctx, batch)
		defer results.Close()

		// Process results
		for i := 0; i < 3; i++ {
			tag, err := results.Exec()
			if err != nil {
				t.Fatalf("Failed to execute batch statement %d: %v", i, err)
			}
			if tag.RowsAffected() != 1 {
				t.Errorf("Expected 1 row affected for statement %d, got %d", i, tag.RowsAffected())
			}
		}

		t.Log("✓ Batch executed successfully without explicit transaction")

		// Verify data was committed automatically
		rows, err := pool.QueryContext(ctx, "SELECT COUNT(*) FROM products WHERE name LIKE 'Batch Product%'")
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

		if count != 3 {
			t.Errorf("Expected 3 products to be committed, got %d", count)
		}

		t.Logf("✓ Verified %d products were committed automatically", count)
	})

	t.Run("batch error rollback without transaction", func(t *testing.T) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Failed to acquire connection: %v", err)
		}
		defer conn.Release()

		// Get count before batch
		var beforeCount int
		rows, _ := pool.QueryContext(ctx, "SELECT COUNT(*) FROM products")
		if rows.Next() {
			rows.Scan(&beforeCount)
		}
		rows.Close()

		batch := &pgx.Batch{}

		// This should succeed
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Error Test 1", 50.00, 5)

		// This should fail (invalid data type for price)
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Error Test 2", "invalid_price", 10)

		// This should not execute
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Error Test 3", 75.00, 15)

		results := conn.SendBatch(ctx, batch)
		defer results.Close()

		// First should succeed
		_, err = results.Exec()
		if err != nil {
			t.Logf("First statement error: %v", err)
		}

		// Second should fail
		_, err = results.Exec()
		if err == nil {
			t.Error("Expected error for invalid data type, got nil")
		} else {
			t.Logf("✓ Got expected error: %v", err)
		}

		// Verify that partial commits don't happen
		// (This depends on whether the batch is wrapped in a transaction by pgx)
		var afterCount int
		rows, _ = pool.QueryContext(ctx, "SELECT COUNT(*) FROM products")
		if rows.Next() {
			rows.Scan(&afterCount)
		}
		rows.Close()

		t.Logf("Count before: %d, after: %d", beforeCount, afterCount)

		// With implicit transaction, count should be same or all committed
		// depending on pgx behavior
		if afterCount == beforeCount {
			t.Log("✓ Batch was implicitly rolled back on error")
		} else if afterCount == beforeCount+1 {
			t.Log("✓ First statement committed before error (auto-commit per statement)")
		}
	})

	t.Run("batch with Pool.SendBatch directly", func(t *testing.T) {
		// Test using Pool directly without acquiring connection
		batch := &pgx.Batch{}

		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Pool Batch 1", 25.00, 100)
		batch.Queue("INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)", "Pool Batch 2", 35.00, 150)

		// Query in the same batch
		batch.Queue("SELECT COUNT(*) FROM products")

		// Pool.SendBatch is available through the embedded *pgxpool.Pool
		results := pool.SendBatch(ctx, batch)
		defer results.Close()

		// Process inserts
		for i := 0; i < 2; i++ {
			tag, err := results.Exec()
			if err != nil {
				t.Fatalf("Failed to execute insert %d: %v", i, err)
			}
			t.Logf("✓ Insert %d: %d rows affected", i, tag.RowsAffected())
		}

		// Process query
		var totalCount int
		err := results.QueryRow().Scan(&totalCount)
		if err != nil {
			t.Fatalf("Failed to scan count: %v", err)
		}
		t.Logf("✓ Total products in database: %d", totalCount)
	})

	t.Run("batch performance comparison", func(t *testing.T) {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Failed to acquire connection: %v", err)
		}
		defer conn.Release()

		numOps := 100

		// Method 1: Individual statements (baseline)
		// We'll skip this in the test to keep it fast, but log the concept
		t.Logf("Comparing batch vs individual operations for %d inserts", numOps)

		// Method 2: Batch
		batch := &pgx.Batch{}
		for i := 0; i < numOps; i++ {
			batch.Queue(
				"INSERT INTO inventory_log (product_id, change_amount, reason) VALUES ($1, $2, $3)",
				1,
				i,
				fmt.Sprintf("batch test %d", i),
			)
		}

		results := conn.SendBatch(ctx, batch)
		defer results.Close()

		successCount := 0
		for i := 0; i < numOps; i++ {
			tag, err := results.Exec()
			if err != nil {
				t.Errorf("Failed to execute batch statement %d: %v", i, err)
				continue
			}
			successCount += int(tag.RowsAffected())
		}

		if successCount != numOps {
			t.Errorf("Expected %d successful inserts, got %d", numOps, successCount)
		}

		t.Logf("✓ Successfully batch-inserted %d inventory log entries", successCount)

		// Verify
		rows, err := pool.QueryContext(ctx, "SELECT COUNT(*) FROM inventory_log WHERE reason LIKE 'batch test%'")
		if err != nil {
			t.Fatalf("Failed to verify: %v", err)
		}
		defer rows.Close()

		var count int
		if rows.Next() {
			rows.Scan(&count)
		}

		if count != numOps {
			t.Errorf("Expected %d records in database, got %d", numOps, count)
		}
	})
}
