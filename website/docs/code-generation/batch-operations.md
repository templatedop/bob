---
sidebar_position: 12
title: Batch Operations with PostgreSQL
description: Complete guide to batch operations from YAML to production API
---

# Batch Operations with PostgreSQL

This guide walks you through using batch operations with Bob and PostgreSQL, from writing queries to deploying in a production API.

## What are Batch Operations?

Batch operations allow you to send multiple SQL statements to the database in a single network round trip. This provides:

- **10-100x faster performance** compared to individual queries
- **Atomic execution** - all statements succeed or all fail
- **Reduced network latency** - single round trip for multiple operations
- **Better resource utilization** - database can optimize related queries

### When to Use Batches

✅ **Good Use Cases:**
- Bulk inserts (inserting 100+ records)
- Multiple related operations (create order + update inventory + log transaction)
- Data migrations
- Batch updates across related tables
- Processing queued items

❌ **Avoid For:**
- Single operations
- Operations that need intermediate results
- Very large batches (>10,000 statements) - split into chunks

## 📚 Related Guides

- **New to batches?** Start with [Quick Start (5 min)](./batch-quick-start)
- **Using generated code?** See [Batches with Generated Code](./batch-with-generated-code)
- **Full example project:** [examples/batch-api](https://github.com/templatedop/bob/tree/main/examples/batch-api)

## Complete Workflow

### Step 1: Write Your SQL Queries

Create `.sql` files in a `queries/` directory with query definitions:

```sql
-- queries/insert_product.sql
-- name: InsertProduct :exec
INSERT INTO products (name, price, stock, category_id)
VALUES ($1, $2, $3, $4);
```

```sql
-- queries/create_order.sql
-- name: CreateOrder :one
INSERT INTO orders (customer_id, total_amount, status)
VALUES ($1, $2, $3)
RETURNING id, created_at;
```

```sql
-- queries/update_inventory.sql
-- name: UpdateInventory :exec
UPDATE products
SET stock = stock - $2
WHERE id = $1;
```

```sql
-- queries/log_transaction.sql
-- name: LogTransaction :exec
INSERT INTO transaction_log (order_id, product_id, quantity, action)
VALUES ($1, $2, $3, $4);
```

### Step 2: Configure Code Generation

Create a `bobgen.yaml` configuration file:

```yaml
# bobgen.yaml
output: generated
pkgname: db
wipe: true
no_tests: false

# PostgreSQL configuration
psql:
  # ✅ REQUIRED: Database connection string
  dsn: "${DATABASE_URL}"

  # ✅ REQUIRED: Folders containing SQL query files
  # This tells Bob where to find your .sql files for code generation
  queries:
    - queries              # Main queries folder
    - api/queries          # Can have multiple folders

  # ✅ REQUIRED: Schemas to generate from
  schemas:
    - public

  # Optional: driver for generated code
  driver: "github.com/jackc/pgx/v5/stdlib"
```

**⚠️ IMPORTANT:** The `queries:` configuration is **REQUIRED** for query code generation!

**Without** `queries:` in YAML:
- ❌ No query functions generated
- ✅ Models still generated from database tables
- ⚠️ You can still use batches, but with manual SQL strings

**With** `queries:` configured:
- ✅ Type-safe query functions generated from `.sql` files
- ✅ Can use generated SQL with batch operations
- ✅ SQL is validated against your database schema

**Environment Variables:**

```bash
# For code generation
export DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable"

# Or use PSQL_DSN
export PSQL_DSN="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
```

### Step 3: Generate Code

Run the code generator:

```bash
# Using go run
go run github.com/templatedop/bob/gen/dopgen-psql@latest -c bobgen.yaml

# Or install globally
go install github.com/templatedop/bob/gen/dopgen-psql@latest
dopgen-psql -c bobgen.yaml
```

This generates Go code in the `generated/` directory:

```
generated/
├── queries/
│   ├── insert_product.bob.go
│   ├── create_order.bob.go
│   ├── update_inventory.bob.go
│   └── log_transaction.bob.go
└── models/
    └── ... (model files)
```

### Step 4: Use in Your Application

#### 4.1: Setup Database Connection

```go
package main

import (
    "context"
    "log"

    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
    "yourapp/generated/queries"
)

func main() {
    ctx := context.Background()

    // Create connection pool
    pool, err := bobpgx.New(ctx, "postgres://user:pass@localhost/mydb")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Use in your API
    api := NewAPI(pool)
    api.Start()
}
```

#### 4.2: Using Batches Without Transactions

**The key insight:** pgx batches are **implicitly transactional** - you don't need explicit `BEGIN/COMMIT`!

```go
func (api *API) BulkCreateProducts(ctx context.Context, products []Product) error {
    // Acquire a connection from the pool
    conn, err := api.pool.Acquire(ctx)
    if err != nil {
        return fmt.Errorf("acquire connection: %w", err)
    }
    defer conn.Release()

    // Create batch - NO explicit transaction needed!
    batch := &pgx.Batch{}

    // Queue all insert statements
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)",
            p.Name, p.Price, p.Stock, p.CategoryID,
        )
    }

    // Send batch - implicitly transactional
    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    // Process results
    for i := range products {
        tag, err := results.Exec()
        if err != nil {
            // On any error, entire batch is rolled back
            return fmt.Errorf("insert product %d: %w", i, err)
        }
        log.Printf("Inserted product: %d rows affected", tag.RowsAffected())
    }

    // All statements committed automatically
    return nil
}
```

#### 4.3: Using Batches With Explicit Transactions

For more complex scenarios, you can combine batches with explicit transactions:

```go
func (api *API) ProcessOrder(ctx context.Context, order Order) error {
    // Begin explicit transaction
    tx, err := api.pool.Begin(ctx)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback(ctx)

    // Create batch within transaction
    batch := &pgx.Batch{}

    // Queue multiple operations
    batch.Queue(
        "INSERT INTO orders (customer_id, total_amount, status) VALUES ($1, $2, $3)",
        order.CustomerID, order.Total, "pending",
    )

    for _, item := range order.Items {
        // Update inventory
        batch.Queue(
            "UPDATE products SET stock = stock - $2 WHERE id = $1",
            item.ProductID, item.Quantity,
        )

        // Log transaction
        batch.Queue(
            "INSERT INTO transaction_log (product_id, quantity, action) VALUES ($1, $2, $3)",
            item.ProductID, item.Quantity, "order",
        )
    }

    // Send batch within transaction
    results := tx.SendBatch(ctx, batch)
    defer results.Close()

    // Process all results
    for i := 0; i < len(order.Items)*2+1; i++ {
        _, err := results.Exec()
        if err != nil {
            return fmt.Errorf("batch operation %d: %w", i, err)
        }
    }

    // Commit transaction
    return tx.Commit(ctx)
}
```

#### 4.4: Mixed Operations (Queries + Inserts)

Batches can contain different operation types:

```go
func (api *API) ProcessInventoryUpdate(ctx context.Context, updates []InventoryUpdate) (*Report, error) {
    conn, err := api.pool.Acquire(ctx)
    if err != nil {
        return nil, err
    }
    defer conn.Release()

    batch := &pgx.Batch{}

    // Get current inventory count
    batch.Queue("SELECT COUNT(*) FROM products WHERE stock < $1", 10)

    // Apply all updates
    for _, update := range updates {
        batch.Queue(
            "UPDATE products SET stock = stock + $2 WHERE id = $1",
            update.ProductID, update.ChangeAmount,
        )
    }

    // Get updated inventory count
    batch.Queue("SELECT COUNT(*) FROM products WHERE stock < $1", 10)

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    // Process query result
    var beforeCount int
    err = results.QueryRow().Scan(&beforeCount)
    if err != nil {
        return nil, fmt.Errorf("before count: %w", err)
    }

    // Process updates
    for i := range updates {
        _, err := results.Exec()
        if err != nil {
            return nil, fmt.Errorf("update %d: %w", i, err)
        }
    }

    // Process final query
    var afterCount int
    err = results.QueryRow().Scan(&afterCount)
    if err != nil {
        return nil, fmt.Errorf("after count: %w", err)
    }

    return &Report{
        BeforeLowStock: beforeCount,
        AfterLowStock:  afterCount,
        UpdatesApplied: len(updates),
    }, nil
}
```

## Real-World API Example

### Complete HTTP API with Batch Operations

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "time"

    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
    "github.com/go-chi/chi/v5"
)

type API struct {
    pool bobpgx.Pool
}

type Product struct {
    Name       string  `json:"name"`
    Price      float64 `json:"price"`
    Stock      int     `json:"stock"`
    CategoryID int     `json:"category_id"`
}

type BulkProductRequest struct {
    Products []Product `json:"products"`
}

type BulkProductResponse struct {
    Inserted int           `json:"inserted"`
    Duration time.Duration `json:"duration_ms"`
    Message  string        `json:"message"`
}

func NewAPI(pool bobpgx.Pool) *API {
    return &API{pool: pool}
}

// POST /api/products/bulk
func (api *API) BulkInsertProducts(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    var req BulkProductRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if len(req.Products) == 0 {
        http.Error(w, "No products provided", http.StatusBadRequest)
        return
    }

    start := time.Now()

    // Acquire connection
    conn, err := api.pool.Acquire(ctx)
    if err != nil {
        http.Error(w, "Database error", http.StatusInternalServerError)
        log.Printf("acquire connection: %v", err)
        return
    }
    defer conn.Release()

    // Create batch
    batch := &pgx.Batch{}
    for _, p := range req.Products {
        batch.Queue(
            "INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)",
            p.Name, p.Price, p.Stock, p.CategoryID,
        )
    }

    // Send batch
    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    // Process results
    inserted := 0
    for i := range req.Products {
        tag, err := results.Exec()
        if err != nil {
            http.Error(w, "Failed to insert products", http.StatusInternalServerError)
            log.Printf("insert product %d: %v", i, err)
            return
        }
        inserted += int(tag.RowsAffected())
    }

    duration := time.Since(start)

    // Return response
    resp := BulkProductResponse{
        Inserted: inserted,
        Duration: duration.Milliseconds(),
        Message:  fmt.Sprintf("Successfully inserted %d products", inserted),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func main() {
    ctx := context.Background()

    // Create connection pool
    pool, err := bobpgx.New(ctx, "postgres://user:pass@localhost/mydb")
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    // Setup API
    api := NewAPI(pool)

    // Setup routes
    r := chi.NewRouter()
    r.Post("/api/products/bulk", api.BulkInsertProducts)

    // Start server
    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", r)
}
```

### Example API Request

```bash
curl -X POST http://localhost:8080/api/products/bulk \
  -H "Content-Type: application/json" \
  -d '{
    "products": [
      {"name": "Widget A", "price": 10.99, "stock": 100, "category_id": 1},
      {"name": "Widget B", "price": 25.50, "stock": 50, "category_id": 1},
      {"name": "Widget C", "price": 15.75, "stock": 75, "category_id": 2}
    ]
  }'
```

**Response:**

```json
{
  "inserted": 3,
  "duration_ms": 12,
  "message": "Successfully inserted 3 products"
}
```

## Performance Comparison

### Without Batch (Individual Inserts)

```go
// ❌ Slow: 100 network round trips
func SlowInsert(ctx context.Context, pool bobpgx.Pool, products []Product) error {
    for _, p := range products {
        _, err := pool.ExecContext(ctx,
            "INSERT INTO products (name, price, stock) VALUES ($1, $2, $3)",
            p.Name, p.Price, p.Stock,
        )
        if err != nil {
            return err
        }
    }
    return nil
}
// Typical time for 100 products: 500-1000ms
```

### With Batch

```go
// ✅ Fast: 1 network round trip
func FastInsert(ctx context.Context, pool bobpgx.Pool, products []Product) error {
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
        _, err := results.Exec()
        if err != nil {
            return err
        }
    }
    return nil
}
// Typical time for 100 products: 10-50ms
```

**Performance Improvement: 10-20x faster!**

## Error Handling Best Practices

### 1. Handle Batch Errors Gracefully

```go
func SafeBatchInsert(ctx context.Context, conn bobpgx.PoolConn, items []Item) error {
    batch := &pgx.Batch{}
    for _, item := range items {
        batch.Queue("INSERT INTO items (name, value) VALUES ($1, $2)", item.Name, item.Value)
    }

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    var firstError error
    for i := range items {
        _, err := results.Exec()
        if err != nil {
            if firstError == nil {
                firstError = fmt.Errorf("item %d (%s): %w", i, items[i].Name, err)
            }
            // Log subsequent errors
            log.Printf("Additional error at item %d: %v", i, err)
        }
    }

    return firstError
}
```

### 2. Validate Before Batching

```go
func ValidatedBatchInsert(ctx context.Context, conn bobpgx.PoolConn, products []Product) error {
    // Validate all items first
    for i, p := range products {
        if p.Name == "" {
            return fmt.Errorf("product %d: name is required", i)
        }
        if p.Price <= 0 {
            return fmt.Errorf("product %d: price must be positive", i)
        }
    }

    // All valid, proceed with batch
    batch := &pgx.Batch{}
    for _, p := range products {
        batch.Queue("INSERT INTO products (name, price) VALUES ($1, $2)", p.Name, p.Price)
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

### 3. Use Timeouts

```go
func BatchWithTimeout(ctx context.Context, conn bobpgx.PoolConn, items []Item) error {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()

    batch := &pgx.Batch{}
    for _, item := range items {
        batch.Queue("INSERT INTO items (data) VALUES ($1)", item.Data)
    }

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    for range items {
        _, err := results.Exec()
        if err != nil {
            if ctx.Err() == context.DeadlineExceeded {
                return fmt.Errorf("batch operation timed out")
            }
            return err
        }
    }

    return nil
}
```

## Advanced Patterns

### 1. Chunked Batching

For very large datasets, split into manageable chunks:

```go
func ChunkedBatchInsert(ctx context.Context, pool bobpgx.Pool, items []Item) error {
    const chunkSize = 1000

    for i := 0; i < len(items); i += chunkSize {
        end := i + chunkSize
        if end > len(items) {
            end = len(items)
        }

        chunk := items[i:end]

        conn, err := pool.Acquire(ctx)
        if err != nil {
            return err
        }

        batch := &pgx.Batch{}
        for _, item := range chunk {
            batch.Queue("INSERT INTO items (data) VALUES ($1)", item.Data)
        }

        results := conn.SendBatch(ctx, batch)
        for range chunk {
            if _, err := results.Exec(); err != nil {
                results.Close()
                conn.Release()
                return err
            }
        }
        results.Close()
        conn.Release()

        log.Printf("Processed chunk %d-%d", i, end)
    }

    return nil
}
```

### 2. Batch with Progress Tracking

```go
type ProgressCallback func(processed, total int)

func BatchWithProgress(ctx context.Context, conn bobpgx.PoolConn,
    items []Item, onProgress ProgressCallback) error {

    batch := &pgx.Batch{}
    for _, item := range items {
        batch.Queue("INSERT INTO items (data) VALUES ($1)", item.Data)
    }

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    for i := range items {
        if _, err := results.Exec(); err != nil {
            return err
        }

        if onProgress != nil {
            onProgress(i+1, len(items))
        }
    }

    return nil
}

// Usage
err := BatchWithProgress(ctx, conn, items, func(processed, total int) {
    log.Printf("Progress: %d/%d (%.1f%%)", processed, total,
        float64(processed)/float64(total)*100)
})
```

### 3. Returning Data from Batch Inserts

```go
type InsertedProduct struct {
    ID        int
    Name      string
    CreatedAt time.Time
}

func BatchInsertWithReturning(ctx context.Context, conn bobpgx.PoolConn,
    products []Product) ([]InsertedProduct, error) {

    batch := &pgx.Batch{}
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock) VALUES ($1, $2, $3) RETURNING id, name, created_at",
            p.Name, p.Price, p.Stock,
        )
    }

    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    inserted := make([]InsertedProduct, 0, len(products))
    for range products {
        var p InsertedProduct
        err := results.QueryRow().Scan(&p.ID, &p.Name, &p.CreatedAt)
        if err != nil {
            return nil, err
        }
        inserted = append(inserted, p)
    }

    return inserted, nil
}
```

## Testing Batch Operations

### Unit Test Example

```go
func TestBulkInsert(t *testing.T) {
    ctx := context.Background()

    // Setup test database
    pool, err := bobpgx.New(ctx, testDSN)
    if err != nil {
        t.Fatal(err)
    }
    defer pool.Close()

    // Clean test data
    _, err = pool.ExecContext(ctx, "TRUNCATE products")
    if err != nil {
        t.Fatal(err)
    }

    // Test data
    products := []Product{
        {Name: "Test 1", Price: 10.0, Stock: 100},
        {Name: "Test 2", Price: 20.0, Stock: 200},
        {Name: "Test 3", Price: 30.0, Stock: 300},
    }

    // Execute batch insert
    err = BulkInsertProducts(ctx, pool, products)
    if err != nil {
        t.Fatalf("BulkInsertProducts failed: %v", err)
    }

    // Verify results
    rows, err := pool.QueryContext(ctx, "SELECT COUNT(*) FROM products")
    if err != nil {
        t.Fatal(err)
    }
    defer rows.Close()

    var count int
    if rows.Next() {
        rows.Scan(&count)
    }

    if count != len(products) {
        t.Errorf("Expected %d products, got %d", len(products), count)
    }
}
```

## Troubleshooting

### Common Issues

#### 1. "connection busy" Error

**Problem:** Trying to use connection while batch is active

```go
// ❌ Wrong
results := conn.SendBatch(ctx, batch)
_, err := conn.ExecContext(ctx, "SELECT 1") // Error: connection busy
results.Close()
```

**Solution:** Always close batch results before using connection again

```go
// ✅ Correct
results := conn.SendBatch(ctx, batch)
// ... process results
results.Close()
_, err := conn.ExecContext(ctx, "SELECT 1") // OK now
```

#### 2. Memory Issues with Large Batches

**Problem:** Running out of memory with very large batches

**Solution:** Use chunked batching (see Advanced Patterns above)

#### 3. Partial Commits

**Problem:** Wondering if partial commits can occur

**Answer:** No! Batches sent via `SendBatch()` are implicitly transactional. Either all statements succeed or all fail (rolled back).

## Best Practices Summary

✅ **DO:**
- Use batches for bulk operations (100+ items)
- Close `results` with `defer results.Close()`
- Validate data before batching
- Use chunking for very large datasets (>10,000 items)
- Handle errors for each statement in the batch
- Use timeouts for long-running batches

❌ **DON'T:**
- Use batches for single operations
- Forget to process all results
- Use the connection while batch is active
- Create batches with >10,000 statements (chunk them)
- Ignore errors from individual statements

## Additional Resources

- [Bob Documentation](https://bob.stephenafamo.com/)
- [pgx Batch Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5#Batch)
- [PostgreSQL Protocol](https://www.postgresql.org/docs/current/protocol.html)
- [Example Code](https://github.com/templatedop/bob/tree/main/test/batch_codegen)

## Next Steps

- [Query Generation Guide](./psql.md)
- [Model Generation](./models.md)
- [Configuration Reference](./configuration.md)
