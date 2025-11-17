---
sidebar_position: 13
title: Batch Operations Quick Start
description: Get started with batch operations in 5 minutes
---

# Batch Operations Quick Start

Get up and running with batch operations in Bob in under 5 minutes!

## What You'll Build

A simple API that inserts 100+ products in a single batch operation, achieving **10-100x better performance** than individual inserts.

## Prerequisites

- Go 1.21+
- PostgreSQL 12+
- 5 minutes ⏱️

## Step 1: Database Setup (1 min)

Start PostgreSQL with Docker:

```bash
docker run --name bob-batch-demo \
  -e POSTGRES_PASSWORD=demo \
  -e POSTGRES_USER=demo \
  -e POSTGRES_DB=demo \
  -p 5432:5432 \
  -d postgres:16-alpine
```

Create schema:

```sql
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Step 2: Write SQL Query (30 seconds)

Create `queries/insert_product.sql`:

```sql
-- name: InsertProduct :one
INSERT INTO products (name, price, stock)
VALUES ($1, $2, $3)
RETURNING id, created_at;
```

## Step 3: Configure Code Generation (30 seconds)

Create `bobgen.yaml`:

```yaml
output: generated
pkgname: db

psql:
  # Database connection
  dsn: "postgres://demo:demo@localhost:5432/demo?sslmode=disable"

  # ⚠️ REQUIRED: Tell Bob where to find .sql query files
  queries:
    - queries   # This folder contains insert_product.sql
```

**📌 Key Point:** The `queries:` line is **required** to generate code from your SQL files!

Without it, Bob only generates models from database tables, not query functions.

## Step 4: Generate Code (30 seconds)

```bash
# Install generator
go install github.com/templatedop/bob/gen/dopgen-psql@latest

# Generate code
dopgen-psql -c bobgen.yaml
```

## Step 5: Use Batch Operations (2 mins)

Create `main.go`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
)

func main() {
    ctx := context.Background()

    // Connect to database
    pool, err := bobpgx.New(ctx, "postgres://demo:demo@localhost:5432/demo?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Prepare products to insert
    products := make([]struct{ Name string; Price float64; Stock int }, 100)
    for i := 0; i < 100; i++ {
        products[i] = struct{ Name string; Price float64; Stock int }{
            Name:  fmt.Sprintf("Product %d", i+1),
            Price: float64(10 + i),
            Stock: 100 + i,
        }
    }

    // Measure time
    start := time.Now()

    // Acquire connection
    conn, err := pool.Acquire(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Release()

    // Create batch - NO explicit transaction needed!
    batch := &pgx.Batch{}
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock) VALUES ($1, $2, $3) RETURNING id",
            p.Name, p.Price, p.Stock,
        )
    }

    // Send batch - implicitly transactional
    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    // Process results
    insertedIDs := make([]int, 0, len(products))
    for range products {
        var id int
        err := results.QueryRow().Scan(&id)
        if err != nil {
            log.Fatalf("Failed to insert: %v", err)
        }
        insertedIDs = append(insertedIDs, id)
    }

    duration := time.Since(start)

    // Print results
    fmt.Printf("✅ Successfully inserted %d products in %dms\n", len(insertedIDs), duration.Milliseconds())
    fmt.Printf("   Performance: %.0f inserts/second\n", float64(len(products))/duration.Seconds())
    fmt.Printf("   First ID: %d, Last ID: %d\n", insertedIDs[0], insertedIDs[len(insertedIDs)-1])
}
```

## Step 6: Run It! (1 min)

```bash
go mod init demo
go mod tidy
go run main.go
```

**Expected Output:**

```
✅ Successfully inserted 100 products in 47ms
   Performance: 2128 inserts/second
   First ID: 1, Last ID: 100
```

## Performance Comparison

### Without Batch (Traditional Approach)

```go
// ❌ Slow: 100 separate database round trips
for _, p := range products {
    _, err := pool.ExecContext(ctx,
        "INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)",
        p.Name, p.Price, p.Stock,
    )
}
// Typical time: 2000-5000ms
```

### With Batch

```go
// ✅ Fast: 1 database round trip
batch := &pgx.Batch{}
for _, p := range products {
    batch.Queue(
        "INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)",
        p.Name, p.Price, p.Stock,
    )
}
results := conn.SendBatch(ctx, batch)
// Typical time: 20-50ms
```

**Result: 40-100x faster!** 🚀

## Key Concepts

### 1. No Explicit Transaction Required

```go
// Batches are implicitly transactional
batch := &pgx.Batch{}
batch.Queue("INSERT ...")
batch.Queue("UPDATE ...")

results := conn.SendBatch(ctx, batch)
// Either ALL succeed or ALL rollback automatically
```

### 2. Mixed Operations

```go
batch := &pgx.Batch{}
batch.Queue("INSERT INTO products ...")  // Insert
batch.Queue("SELECT COUNT(*) ...")       // Query
batch.Queue("UPDATE inventory ...")      // Update
batch.Queue("DELETE FROM temp ...")      // Delete

results := conn.SendBatch(ctx, batch)
```

### 3. Returning Data

```go
batch := &pgx.Batch{}
for _, p := range products {
    batch.Queue(
        "INSERT INTO products (name) VALUES ($1) RETURNING id, created_at",
        p.Name,
    )
}

results := conn.SendBatch(ctx, batch)
for range products {
    var id int
    var createdAt time.Time
    results.QueryRow().Scan(&id, &createdAt)
}
```

## Common Use Cases

### Bulk Product Import

```go
func ImportProducts(ctx context.Context, pool bobpgx.Pool, csvFile string) error {
    products := parseCSV(csvFile) // Your CSV parser

    conn, _ := pool.Acquire(ctx)
    defer conn.Release()

    batch := &pgx.Batch{}
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)",
            p.Name, p.Price, p.Stock,
        )
    }

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    for range products {
        if _, err := results.Exec(); err != nil {
            return err
        }
    }

    return nil
}
```

### Order Processing

```go
func ProcessOrder(ctx context.Context, pool bobpgx.Pool, order Order) error {
    conn, _ := pool.Acquire(ctx)
    defer conn.Release()

    batch := &pgx.Batch{}

    // Create order
    batch.Queue(
        "INSERT INTO orders (customer_id, total) VALUES ($1, $2)",
        order.CustomerID, order.Total,
    )

    // Update inventory for each item
    for _, item := range order.Items {
        batch.Queue(
            "UPDATE products SET stock = stock - $2 WHERE id = $1",
            item.ProductID, item.Quantity,
        )
    }

    // Log transaction
    batch.Queue(
        "INSERT INTO transaction_log (order_id, action) VALUES ($1, $2)",
        order.ID, "created",
    )

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    // Process all results
    for i := 0; i < len(order.Items)+2; i++ {
        if _, err := results.Exec(); err != nil {
            return err
        }
    }

    return nil
}
```

## Next Steps

Now that you've seen the basics:

1. **[Complete Guide](./batch-operations)** - Learn all batch operation patterns
2. **[Full Example Project](https://github.com/templatedop/bob/tree/main/examples/batch-api)** - Complete REST API with batching
3. **[Error Handling](./batch-operations#error-handling-best-practices)** - Production-ready error handling
4. **[Advanced Patterns](./batch-operations#advanced-patterns)** - Chunking, progress tracking, etc.

## Troubleshooting

**"connection busy" error?**
- Make sure to `defer results.Close()` after `SendBatch()`

**Running out of memory?**
- Use chunking for very large datasets (see [Advanced Patterns](./batch-operations#advanced-patterns))

**Need explicit transaction control?**
- You can still use `tx.Begin()` and wrap batches in explicit transactions

## Clean Up

```bash
docker stop bob-batch-demo
docker rm bob-batch-demo
```

## Summary

✅ You've learned how to:
- Set up batch operations with Bob
- Configure YAML with `queries:` (required for code generation)
- Insert 100+ records in <50ms
- Achieve 10-100x performance improvement
- Use batches without explicit transactions

**📌 Key Point:** The `queries:` configuration in `bobgen.yaml` is **required** to generate code from your `.sql` files. Without it, you'll only get model generation, not query functions!

**Ready for production?** Check out:
- [Complete Guide](./batch-operations) - All features and patterns
- [Batches with Generated Code](./batch-with-generated-code) - Integration guide
- [Example Project](https://github.com/templatedop/bob/tree/main/examples/batch-api) - Full REST API
