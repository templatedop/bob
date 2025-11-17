---
sidebar_position: 14
title: Using Generated Code with Batches
description: How to use Bob's generated query code with batch operations
---

# Using Generated Code with Batch Operations

This guide shows you how to combine Bob's code generation with batch operations for maximum performance and type safety.

## Understanding the Integration

Bob generates type-safe query functions from your SQL files, and you use **pgx.Batch** to execute them efficiently:

```
Your SQL Queries → dopgen-psql → Generated Functions → pgx.Batch → Fast Execution
```

## Step-by-Step Workflow

### Step 1: Write SQL Queries with Proper Annotations

Create query files with Bob's standard annotations:

```sql
-- queries/insert_product.sql
-- name: InsertProduct :exec
INSERT INTO products (name, price, stock, category_id)
VALUES ($1, $2, $3, $4);
```

```sql
-- queries/update_inventory.sql
-- name: UpdateInventory :exec
UPDATE products
SET stock = stock - $2
WHERE id = $1;
```

```sql
-- queries/get_product.sql
-- name: GetProduct :one
SELECT id, name, price, stock, category_id, created_at
FROM products
WHERE id = $1;
```

**Query Annotations:**
- `:exec` - Execute without returning data (INSERT, UPDATE, DELETE)
- `:one` - Return single row (SELECT with RETURNING)
- `:many` - Return multiple rows (SELECT queries)

### Step 2: Configure YAML for Query Generation

Create `bobgen.yaml`:

```yaml
output: generated
pkgname: db
wipe: true
no_tests: false

psql:
  # Database connection
  dsn: "${DATABASE_URL}"

  # IMPORTANT: Specify folders containing query files
  queries:
    - queries        # Path to your .sql files
    - api/queries    # You can have multiple query folders

  # Optional: schemas to include
  schemas:
    - public

  # Optional: driver for generated code
  driver: "github.com/jackc/pgx/v5/stdlib"
```

**Key Configuration:**
- `queries:` - **Required for query code generation**
- List all folders containing `.sql` query files
- Bob will scan these folders and generate Go code

### Step 3: Generate Code

```bash
# Install generator
go install github.com/templatedop/bob/gen/dopgen-psql@latest

# Generate code from queries
dopgen-psql -c bobgen.yaml
```

This generates:

```
generated/
├── queries/
│   ├── insert_product.bob.go
│   ├── update_inventory.bob.go
│   └── get_product.bob.go
└── models/
    └── ... (model files)
```

### Step 4: Use Generated Queries with Batches

Now you have two options for batch operations:

#### Option A: Direct Batch with SQL Strings

```go
package main

import (
    "context"
    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
)

func BulkInsertProducts(ctx context.Context, pool bobpgx.Pool, products []Product) error {
    conn, err := pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    // Create batch
    batch := &pgx.Batch{}

    // Queue the same SQL that Bob generated
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)",
            p.Name, p.Price, p.Stock, p.CategoryID,
        )
    }

    // Execute batch
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
```

#### Option B: Extract SQL from Generated Code (Recommended)

Bob generates query constants you can reuse:

```go
package main

import (
    "context"
    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
    "yourapp/generated/queries"
)

func BulkInsertProducts(ctx context.Context, pool bobpgx.Pool, products []Product) error {
    conn, err := pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    batch := &pgx.Batch{}

    // Use the SQL string from generated code
    // (Check your generated file for the exact constant name)
    insertSQL := `INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)`

    for _, p := range products {
        batch.Queue(insertSQL, p.Name, p.Price, p.Stock, p.CategoryID)
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

#### Option C: Mixed - Generated Queries + Batch

Use generated functions for single operations, batch for bulk:

```go
package main

import (
    "context"
    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
    "yourapp/generated/queries"
)

type ProductService struct {
    pool bobpgx.Pool
}

// Single insert - use generated function
func (s *ProductService) CreateProduct(ctx context.Context, p Product) error {
    // Use Bob's type-safe generated function for single operations
    return queries.InsertProduct(ctx, s.pool, p.Name, p.Price, p.Stock, p.CategoryID)
}

// Bulk insert - use batch
func (s *ProductService) BulkCreateProducts(ctx context.Context, products []Product) error {
    conn, err := s.pool.Acquire(ctx)
    if err != nil {
        return err
    }
    defer conn.Release()

    batch := &pgx.Batch{}
    for _, p := range products {
        // Queue using the same SQL as generated function
        batch.Queue(
            "INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)",
            p.Name, p.Price, p.Stock, p.CategoryID,
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

## Complete Example: Order Processing

### Queries

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

### YAML Configuration

```yaml
output: generated
pkgname: db

psql:
  dsn: "${DATABASE_URL}"
  queries:
    - queries  # ← CRITICAL: tells Bob where to find query files
```

### Generate

```bash
export DATABASE_URL="postgres://user:pass@localhost/mydb"
dopgen-psql -c bobgen.yaml
```

### Use with Batches

```go
package main

import (
    "context"
    "fmt"
    "github.com/jackc/pgx/v5"
    bobpgx "github.com/templatedop/bob/drivers/pgx"
)

type Order struct {
    CustomerID int
    Total      float64
    Items      []OrderItem
}

type OrderItem struct {
    ProductID int
    Quantity  int
}

func ProcessOrder(ctx context.Context, pool bobpgx.Pool, order Order) error {
    // Start explicit transaction for complex operation
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // Create batch for multiple operations
    batch := &pgx.Batch{}

    // 1. Create order
    batch.Queue(
        "INSERT INTO orders (customer_id, total_amount, status) VALUES ($1, $2, $3) RETURNING id",
        order.CustomerID, order.Total, "pending",
    )

    // 2. Update inventory for each item
    for _, item := range order.Items {
        batch.Queue(
            "UPDATE products SET stock = stock - $2 WHERE id = $1",
            item.ProductID, item.Quantity,
        )
    }

    // 3. Log all transactions
    for _, item := range order.Items {
        batch.Queue(
            "INSERT INTO transaction_log (product_id, quantity, action) VALUES ($1, $2, $3)",
            item.ProductID, item.Quantity, "order",
        )
    }

    // Execute all operations as a batch
    results := tx.SendBatch(ctx, batch)
    defer results.Close()

    // Get order ID
    var orderID int
    err = results.QueryRow().Scan(&orderID)
    if err != nil {
        return fmt.Errorf("create order: %w", err)
    }

    // Process inventory updates
    for range order.Items {
        _, err := results.Exec()
        if err != nil {
            return fmt.Errorf("update inventory: %w", err)
        }
    }

    // Process transaction logs
    for range order.Items {
        _, err := results.Exec()
        if err != nil {
            return fmt.Errorf("log transaction: %w", err)
        }
    }

    // Commit transaction
    return tx.Commit(ctx)
}
```

## Important Configuration Notes

### Required YAML Settings

```yaml
psql:
  # ✅ REQUIRED: Database connection for introspection
  dsn: "${DATABASE_URL}"

  # ✅ REQUIRED: Folders with .sql query files
  queries:
    - queries
    - another/query/folder

  # ✅ REQUIRED: Schemas to generate from
  schemas:
    - public
    - tenant_schema
```

### Without `queries:` Configuration

If you omit `queries:` from YAML:
- ❌ No query code generation
- ✅ Model code still generated
- ✅ Can still use batches with manual SQL

```yaml
# This only generates models, NOT query functions
psql:
  dsn: "${DATABASE_URL}"
  # queries: []  ← Missing = no query generation
```

## Best Practices

### 1. Use Generated Code for Single Operations

```go
// ✅ Single product creation - use generated function
err := queries.InsertProduct(ctx, pool, name, price, stock, categoryID)

// ✅ Bulk creation - use batch
batch := &pgx.Batch{}
for _, p := range products {
    batch.Queue("INSERT INTO products (...) VALUES (...)", p.Name, p.Price, ...)
}
```

### 2. Keep SQL Synchronized

```go
// ❌ BAD: SQL drift from generated code
batch.Queue("INSERT INTO products (name, price) VALUES ($1, $2)", ...)

// ✅ GOOD: Match generated SQL exactly
// Copy from generated/queries/insert_product.bob.go
batch.Queue("INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)", ...)
```

### 3. Type Safety with Generated Types

```go
import "yourapp/generated/models"

// Use generated types for type safety
type ProductService struct {
    pool bobpgx.Pool
}

func (s *ProductService) BulkInsert(ctx context.Context, products []models.Product) error {
    conn, _ := s.pool.Acquire(ctx)
    defer conn.Release()

    batch := &pgx.Batch{}
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4)",
            p.Name, p.Price, p.Stock, p.CategoryID,
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

## Troubleshooting

### "No query code generated"

**Problem:** Generated code doesn't include query functions

**Solution:** Add `queries:` to YAML configuration

```yaml
psql:
  dsn: "${DATABASE_URL}"
  queries:          # ← Add this
    - queries       # ← Point to your .sql files
```

### "Cannot find query SQL constant"

**Problem:** Looking for constant in generated code

**Solution:** Bob generates functions, not constants. Extract SQL from `.sql` file or generated function:

```go
// Option 1: Read from your .sql file
const insertProductSQL = "INSERT INTO products ..." // from insert_product.sql

// Option 2: Check generated code comments
// Generated files often include the SQL in comments
```

### "Batch slower than expected"

**Problem:** Not seeing performance improvement

**Checklist:**
- ✅ Using `conn.SendBatch()` not individual `Exec()`?
- ✅ Processing all results in the batch?
- ✅ Not in auto-commit mode?
- ✅ Batch size appropriate (100-10000)?

## Summary

**YAML Configuration for Batches:**

```yaml
output: generated
pkgname: db

psql:
  dsn: "${DATABASE_URL}"        # Required
  queries:                      # Required for query generation
    - queries                   # Your .sql files
  schemas:                      # Required
    - public
```

**Workflow:**

1. Write `.sql` files with `-- name:` annotations
2. Add `queries:` folders to `bobgen.yaml`
3. Run `dopgen-psql -c bobgen.yaml`
4. Use generated types + `pgx.Batch` for bulk operations

**Key Insight:**
- Bob generates **type-safe query functions** for single operations
- You use **pgx.Batch** manually for bulk operations
- SQL from `.sql` files is used in both

## See Also

- [Batch Operations Guide](./batch-operations) - Complete batch operations reference
- [Query Generation](./psql#queries) - Bob query generation details
- [Configuration Reference](./configuration) - All YAML options
