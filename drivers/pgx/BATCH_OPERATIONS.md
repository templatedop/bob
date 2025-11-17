# Batch Operations with pgx Driver

## Overview

The Bob pgx driver fully supports PostgreSQL batch operations through the `SendBatch` method available on transactions. Batch operations allow you to send multiple queries to the database in a single round trip, significantly improving performance when executing multiple statements.

## Features

- ✅ Full support for `pgx.Batch` and `SendBatch` methods
- ✅ Works with `Pool`, `PoolConn`, `Conn`, and `Tx` types
- ✅ Supports mixed operations (INSERT, UPDATE, DELETE, SELECT) in a single batch
- ✅ Proper error handling and transaction management
- ✅ Compatible with all pgx/v5 features

## Basic Usage

### Creating and Executing a Batch

```go
package main

import (
    "context"
    "log"

    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
)

func main() {
    ctx := context.Background()

    // Create a connection pool
    pool, err := bobpgx.New(ctx, "postgres://user:pass@localhost/dbname")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Begin a transaction
    tx, err := pool.Begin(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer tx.Rollback(ctx)

    // Create a batch
    batch := &pgx.Batch{}

    // Queue multiple statements
    batch.Queue("INSERT INTO users (name, email) VALUES ($1, $2)", "Alice", "alice@example.com")
    batch.Queue("INSERT INTO users (name, email) VALUES ($1, $2)", "Bob", "bob@example.com")
    batch.Queue("INSERT INTO users (name, email) VALUES ($1, $2)", "Charlie", "charlie@example.com")

    // Send the batch
    results := tx.SendBatch(ctx, batch)
    defer results.Close()

    // Process results
    for i := 0; i < 3; i++ {
        tag, err := results.Exec()
        if err != nil {
            log.Printf("Statement %d failed: %v", i, err)
            return
        }
        log.Printf("Statement %d affected %d rows", i, tag.RowsAffected())
    }

    // Commit the transaction
    if err := tx.Commit(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### Mixed Operations

You can mix different types of operations in a single batch:

```go
batch := &pgx.Batch{}

// Insert
batch.Queue("INSERT INTO products (name, price) VALUES ($1, $2)", "Widget", 9.99)

// Query
batch.Queue("SELECT COUNT(*) FROM products WHERE price > $1", 5.00)

// Update
batch.Queue("UPDATE products SET price = price * 1.1 WHERE price < $1", 10.00)

// Delete
batch.Queue("DELETE FROM products WHERE price > $1", 100.00)

results := tx.SendBatch(ctx, batch)
defer results.Close()

// Process insert
tag, err := results.Exec()
// ... handle result

// Process query
var count int
err = results.QueryRow().Scan(&count)
// ... handle result

// Process update
tag, err = results.Exec()
// ... handle result

// Process delete
tag, err = results.Exec()
// ... handle result
```

## Performance Benefits

Batch operations provide significant performance improvements:

1. **Reduced Network Round Trips**: All queries are sent in a single network call
2. **Better Resource Utilization**: Database can optimize execution of related queries
3. **Transaction Efficiency**: Multiple operations within a single transaction boundary

### Example: Bulk Insert Performance

```go
// Traditional approach - 1000 round trips
for i := 0; i < 1000; i++ {
    _, err := tx.Exec(ctx, "INSERT INTO items (value) VALUES ($1)", i)
    // ... handle error
}

// Batch approach - 1 round trip
batch := &pgx.Batch{}
for i := 0; i < 1000; i++ {
    batch.Queue("INSERT INTO items (value) VALUES ($1)", i)
}
results := tx.SendBatch(ctx, batch)
defer results.Close()

for i := 0; i < 1000; i++ {
    _, err := results.Exec()
    // ... handle error
}
```

## Error Handling

When a statement in a batch fails, subsequent statements are not executed, and the transaction can be rolled back:

```go
batch := &pgx.Batch{}
batch.Queue("INSERT INTO users (name) VALUES ($1)", "User1")
batch.Queue("INSERT INTO users (name) VALUES ($1)", nil) // Will fail - NULL constraint
batch.Queue("INSERT INTO users (name) VALUES ($1)", "User3") // Won't execute

results := tx.SendBatch(ctx, batch)
defer results.Close()

// First succeeds
tag, err := results.Exec()
if err != nil {
    log.Printf("First statement failed: %v", err)
}

// Second fails
tag, err = results.Exec()
if err != nil {
    log.Printf("Second statement failed (expected): %v", err)
    // Rollback the transaction
    tx.Rollback(ctx)
    return
}

// Third won't be reached due to error
```

## Using with Different Connection Types

### Pool Connection

```go
conn, err := pool.Acquire(ctx)
if err != nil {
    log.Fatal(err)
}
defer conn.Release()

tx, err := conn.Begin(ctx)
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback(ctx)

batch := &pgx.Batch{}
// ... add statements

results := tx.SendBatch(ctx, batch)
defer results.Close()
// ... process results
```

### Standalone Connection

```go
pgxConn, err := pgx.Connect(ctx, dsn)
if err != nil {
    log.Fatal(err)
}
defer pgxConn.Close(ctx)

conn := bobpgx.NewConn(pgxConn)
tx, err := conn.Begin(ctx)
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback(ctx)

batch := &pgx.Batch{}
// ... add statements

results := tx.SendBatch(ctx, batch)
defer results.Close()
// ... process results
```

## Testing

The pgx driver includes comprehensive tests for batch operations:

```bash
# Run unit tests (skip if no database connection)
go test ./drivers/pgx/...

# Run integration tests (requires Docker)
go test -tags=integration ./drivers/pgx/...
```

To run tests with a real database, set the `PGX_TEST_DSN` environment variable:

```bash
export PGX_TEST_DSN="postgres://user:pass@localhost/testdb?sslmode=disable"
go test -v ./drivers/pgx/... -run TestBatch
```

## Best Practices

1. **Always use transactions**: Batch operations should be executed within a transaction for consistency
2. **Close results**: Always defer `results.Close()` to free resources
3. **Handle errors**: Check errors for each statement in the batch
4. **Reasonable batch sizes**: While pgx can handle large batches, consider breaking very large operations into multiple batches (e.g., 1000-10000 statements per batch)
5. **Statement order**: Put queries that are least likely to fail first for better error recovery

## Compatibility

- **Bob version**: All versions with pgx driver support
- **pgx version**: v5.x (tested with v5.7.5+)
- **PostgreSQL**: 12+

## Additional Resources

- [pgx Batch Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5#Batch)
- [Bob Documentation](https://github.com/templatedop/bob)
- [PostgreSQL Protocol Documentation](https://www.postgresql.org/docs/current/protocol.html)
