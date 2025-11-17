# Batch Operations API Example

This is a complete, runnable example of using Bob's batch operations in a production REST API.

## What This Demonstrates

- ✅ **Complete workflow**: YAML → Code Generation → Production API
- ✅ **Batch operations**: Bulk inserts with 10-100x performance improvement
- ✅ **Real HTTP API**: Complete REST endpoints with proper error handling
- ✅ **Best practices**: Validation, timeouts, progress tracking
- ✅ **Docker setup**: PostgreSQL database with initial schema

## Quick Start

### 1. Start PostgreSQL

```bash
docker-compose up -d
```

This starts PostgreSQL on `localhost:5432` with:
- Database: `batchapi`
- Username: `batchuser`
- Password: `batchpass`

### 2. Generate Code

```bash
# Install code generator
go install github.com/templatedop/bob/gen/dopgen-psql@latest

# Generate from queries (uses bobgen.yaml)
dopgen-psql -c bobgen.yaml
```

**⚠️ Note:** The `bobgen.yaml` file includes `queries: [queries]` which tells Bob to generate code from the `.sql` files in the `queries/` folder. This is **required** for query code generation!

```yaml
# bobgen.yaml (already configured)
psql:
  dsn: "${DATABASE_URL}"
  queries:              # ← REQUIRED for batch operations
    - queries           # ← Points to queries/*.sql files
```

### 3. Run the API

```bash
go run main.go
```

Server starts on `http://localhost:8080`

### 4. Try It Out

**Bulk Insert Products:**

```bash
curl -X POST http://localhost:8080/api/products/bulk \
  -H "Content-Type: application/json" \
  -d '{
    "products": [
      {"name": "Laptop", "price": 999.99, "stock": 50, "category_id": 1},
      {"name": "Mouse", "price": 29.99, "stock": 200, "category_id": 2},
      {"name": "Keyboard", "price": 79.99, "stock": 150, "category_id": 2}
    ]
  }'
```

**Response:**
```json
{
  "inserted": 3,
  "duration_ms": 15,
  "message": "Successfully inserted 3 products",
  "products": [
    {
      "id": 1,
      "name": "Laptop",
      "price": 999.99,
      "stock": 50,
      "category_id": 1,
      "created_at": "2025-11-15T12:34:56Z"
    },
    ...
  ]
}
```

**Get All Products:**

```bash
curl http://localhost:8080/api/products
```

**Performance Test** (insert 1000 products):

```bash
curl -X POST http://localhost:8080/api/products/bulk/test?count=1000
```

## Project Structure

```
batch-api/
├── docker-compose.yml          # PostgreSQL setup
├── bobgen.yaml                 # Code generation config
├── schema.sql                  # Database schema
├── queries/                    # SQL query files
│   ├── insert_product.sql
│   ├── get_product.sql
│   └── list_products.sql
├── generated/                  # Generated code (git ignored)
│   └── queries/
├── main.go                     # HTTP server
└── README.md                   # This file
```

## API Endpoints

### POST /api/products/bulk

Bulk insert products using batch operations.

**Request:**
```json
{
  "products": [
    {
      "name": "Product Name",
      "price": 99.99,
      "stock": 100,
      "category_id": 1
    }
  ]
}
```

**Response:**
```json
{
  "inserted": 1,
  "duration_ms": 12,
  "message": "Successfully inserted 1 products",
  "products": [...]
}
```

### GET /api/products

List all products.

**Response:**
```json
{
  "products": [...],
  "count": 10
}
```

### GET /api/products/:id

Get single product by ID.

### POST /api/products/bulk/test

Performance test endpoint. Generates and inserts N products.

**Query Parameters:**
- `count` - Number of products to insert (default: 100)

## Performance Comparison

The API includes an endpoint to demonstrate batch performance:

```bash
# Insert 1000 products using batch
curl -X POST 'http://localhost:8080/api/products/bulk/test?count=1000'
```

**Typical Results:**
- Batch: **50-100ms** for 1000 inserts (1 network round trip)
- Individual: **5000-10000ms** for 1000 inserts (1000 network round trips)
- **Performance improvement: ~100x faster**

## Key Code Snippets

### Batch Insert Function

```go
func (api *API) BulkInsertProducts(ctx context.Context, products []Product) ([]InsertedProduct, error) {
    conn, err := api.pool.Acquire(ctx)
    if err != nil {
        return nil, err
    }
    defer conn.Release()

    // Create batch - NO explicit transaction needed!
    batch := &pgx.Batch{}
    for _, p := range products {
        batch.Queue(
            "INSERT INTO products (name, price, stock, category_id) VALUES ($1, $2, $3, $4) RETURNING id, created_at",
            p.Name, p.Price, p.Stock, p.CategoryID,
        )
    }

    // Send batch - implicitly transactional
    results := conn.SendBatch(ctx, batch)
    defer results.Close()

    // Collect results
    inserted := make([]InsertedProduct, 0, len(products))
    for i, p := range products {
        var id int
        var createdAt time.Time
        err := results.QueryRow().Scan(&id, &createdAt)
        if err != nil {
            return nil, fmt.Errorf("insert product %d: %w", i, err)
        }
        inserted = append(inserted, InsertedProduct{
            ID:         id,
            Name:       p.Name,
            Price:      p.Price,
            Stock:      p.Stock,
            CategoryID: p.CategoryID,
            CreatedAt:  createdAt,
        })
    }

    return inserted, nil
}
```

## Environment Variables

```bash
# Database connection
DATABASE_URL="postgres://batchuser:batchpass@localhost:5432/batchapi?sslmode=disable"

# Server configuration
PORT=8080
```

## Cleanup

```bash
# Stop and remove containers
docker-compose down

# Remove volumes (database data)
docker-compose down -v
```

## Learn More

- [Batch Operations Documentation](../../website/docs/code-generation/batch-operations.md)
- [Bob Documentation](https://bob.stephenafamo.com/)
- [pgx Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5)

## License

This example is part of the Bob project and follows the same license.
