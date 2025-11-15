# Batch Operations Code Generation Tests

This directory contains comprehensive tests for pgx batch operations with Bob, including:

1. **Query-driven code generation** - Demonstrates generating Go code from SQL queries
2. **Batch operations without transactions** - Shows that pgx batches are implicitly transactional
3. **Performance testing** - Compares batch vs individual operations
4. **Error handling** - Tests rollback behavior in batches

## Directory Structure

```
batch_codegen/
├── migrations/
│   └── 001_create_test_tables.sql   # Database schema
├── queries/
│   ├── insert_product.sql           # Query: Insert product
│   ├── create_order.sql             # Query: Create order
│   ├── log_inventory.sql            # Query: Log inventory change
│   ├── update_stock.sql             # Query: Update product stock
│   ├── get_product.sql              # Query: Get single product
│   └── list_low_stock.sql           # Query: List low stock products
├── generated/                       # Generated code output
├── bobgen.yaml                      # Code generation config
├── codegen_test.go                  # Test file
└── README.md                        # This file
```

## Running Tests

### Prerequisites

Set up a PostgreSQL database for testing:

```bash
export PGX_TEST_DSN="postgres://user:pass@localhost/testdb?sslmode=disable"
```

Or use Docker:

```bash
docker run --name bob-test-pg \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_USER=testuser \
  -e POSTGRES_DB=testdb \
  -p 5432:5432 \
  -d postgres:16-alpine

export PGX_TEST_DSN="postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable"
```

### Run Tests

```bash
# From the bob root directory
cd test/batch_codegen

# Run all tests
go test -v .

# Run specific test
go test -v . -run TestBatchWithoutTransaction

# Run code generation test
go test -v . -run TestBatchCodeGeneration
```

## What These Tests Demonstrate

### 1. Batch Without Explicit Transaction

Tests that pgx batches work without wrapping in `BEGIN/COMMIT`:

```go
// No tx.Begin() needed!
batch := &pgx.Batch{}
batch.Queue("INSERT INTO products (...) VALUES (...)")
batch.Queue("INSERT INTO products (...) VALUES (...)")

results := conn.SendBatch(ctx, batch)
// All statements execute atomically
```

**Key Points:**
- Batches are implicitly transactional
- Either all statements succeed or all rollback
- No need for explicit transaction management
- Connection handles commit/rollback automatically

### 2. Error Handling in Batches

Shows what happens when a statement in a batch fails:

```go
batch := &pgx.Batch{}
batch.Queue("INSERT INTO products (...) VALUES (...)")      // ✓ succeeds
batch.Queue("INSERT INTO products (...) VALUES (INVALID)")  // ✗ fails
batch.Queue("INSERT INTO products (...) VALUES (...)")      // ✗ not executed

results := conn.SendBatch(ctx, batch)
// First statement may commit (auto-commit mode)
// OR entire batch rolls back (transaction mode)
```

### 3. Pool vs Connection vs Transaction

Tests batch operations with different pgx types:

- `Pool.SendBatch()` - Direct pool usage
- `PoolConn.SendBatch()` - Acquired connection
- `Conn.SendBatch()` - Standalone connection
- `Tx.SendBatch()` - Within explicit transaction

### 4. Performance Benefits

Demonstrates the performance advantage of batching:

- **Individual statements**: 100 round trips for 100 inserts
- **Batch**: 1 round trip for 100 inserts

Typical improvement: **10-100x faster** depending on network latency

### 5. Mixed Operations

Shows batches can contain different operation types:

```go
batch := &pgx.Batch{}
batch.Queue("INSERT INTO ...")     // Insert
batch.Queue("SELECT COUNT(*) ...") // Query
batch.Queue("UPDATE ...")          // Update
batch.Queue("DELETE FROM ...")     // Delete

results := conn.SendBatch(ctx, batch)
// Process each result in order
```

## Code Generation Workflow

The `TestBatchCodeGeneration` test demonstrates the complete workflow:

1. **Create schema** - Run migrations to set up tables
2. **Write queries** - Define SQL queries in `queries/*.sql`
3. **Configure** - Set up `bobgen.yaml` with query paths
4. **Generate** - Run `dopgen-psql` to generate Go code
5. **Use** - Import and use generated query functions

### Generated Code Example

From `queries/insert_product.sql`:

```sql
-- name: InsertProduct :exec
INSERT INTO products (name, price, stock)
VALUES ($1, $2, $3);
```

Generates (approximately):

```go
func InsertProduct(ctx context.Context, exec bob.Executor, name string, price decimal.Decimal, stock int) error {
    // ... generated implementation
}
```

## Testing Strategy

1. **Unit Tests** - Test individual batch operations
2. **Integration Tests** - Test with real PostgreSQL database
3. **Error Cases** - Test rollback and error handling
4. **Performance** - Verify batch performance benefits
5. **Code Generation** - Test end-to-end code generation workflow

## Expected Results

All tests should pass when run against a PostgreSQL database:

```
=== RUN   TestBatchCodeGeneration
--- PASS: TestBatchCodeGeneration

=== RUN   TestBatchWithoutTransaction
=== RUN   TestBatchWithoutTransaction/batch_without_explicit_transaction
--- PASS: TestBatchWithoutTransaction/batch_without_explicit_transaction
=== RUN   TestBatchWithoutTransaction/batch_error_rollback
--- PASS: TestBatchWithoutTransaction/batch_error_rollback
=== RUN   TestBatchWithoutTransaction/batch_with_Pool.SendBatch_directly
--- PASS: TestBatchWithoutTransaction/batch_with_Pool.SendBatch_directly
=== RUN   TestBatchWithoutTransaction/batch_performance_comparison
--- PASS: TestBatchWithoutTransaction/batch_performance_comparison
--- PASS: TestBatchWithoutTransaction

PASS
```

## Cleanup

To stop and remove the Docker container:

```bash
docker stop bob-test-pg
docker rm bob-test-pg
```

## Additional Resources

- [pgx Batch Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5#Batch)
- [Bob pgx Driver](../../drivers/pgx/)
- [Bob Documentation](https://bob.stephenafamo.com/)
