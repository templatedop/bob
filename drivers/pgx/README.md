# Bob PostgreSQL Driver (pgx)

This package provides **native PostgreSQL support** for Bob ORM using the [jackc/pgx/v5](https://github.com/jackc/pgx) driver.

## 🎯 This is the ONLY Recommended PostgreSQL Driver for Bob

**Important**: While Bob technically works with other PostgreSQL drivers (`lib/pq`, `pgx/stdlib`), we **strongly recommend using this native pgx driver** for all PostgreSQL projects.

### Why pgx Native Only?

1. ✅ **Batch Operations** - Only pgx native supports batch operations (N queries in 1 round trip)
2. ✅ **Best Performance** - Binary protocol, optimized for PostgreSQL
3. ✅ **Full PostgreSQL Support** - Arrays, JSON, COPY, Listen/Notify, UUID, etc.
4. ✅ **Best Connection Pooling** - `pgxpool` with detailed statistics
5. ✅ **Active Development** - Maintained by PostgreSQL experts
6. ✅ **Production Proven** - Used by thousands of Go applications

### ⚠️ Other Drivers (NOT Recommended)

| Driver | Status | Issue |
|--------|--------|-------|
| `lib/pq` | ❌ **Deprecated** | Maintenance mode, no batch support, slower |
| `pgx/stdlib` | ⚠️ **Limited** | No batch support, missing features |

**Bottom Line**: If you're using PostgreSQL, use this driver. Period.

---

## Installation

```bash
go get github.com/stephenafamo/bob/drivers/pgx
go get github.com/jackc/pgx/v5/pgxpool
```

## Quick Start

### Basic Connection

```go
import (
    "context"
    "github.com/stephenafamo/bob/drivers/pgx"
)

func main() {
    ctx := context.Background()

    // Create connection pool
    pool, err := pgx.New(ctx, "postgres://user:pass@localhost:5432/mydb")
    if err != nil {
        panic(err)
    }
    defer pool.Close()

    // Use with Bob queries
    users, _ := models.Users.Query().All(ctx, pool)
}
```

### With Connection Pool Configuration

```go
import (
    "github.com/stephenafamo/bob/drivers/pgx"
    "github.com/jackc/pgx/v5/pgxpool"
    "time"
)

func main() {
    ctx := context.Background()

    // Parse config
    config, err := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/mydb")
    if err != nil {
        panic(err)
    }

    // Configure pool
    config.MaxConns = 25
    config.MinConns = 5
    config.MaxConnLifetime = time.Hour
    config.MaxConnIdleTime = 30 * time.Minute
    config.HealthCheckPeriod = time.Minute

    // Create pool
    pool, err := pgx.NewWithConfig(ctx, config)
    if err != nil {
        panic(err)
    }
    defer pool.Close()

    // Use with Bob
    users, _ := models.Users.Query().All(ctx, pool)
}
```

## Features

### ✅ Connection Pooling

```go
pool, _ := pgx.New(ctx, dsn)

// Pool statistics
stats := pool.Stat()
fmt.Printf("Total: %d, Idle: %d, Acquired: %d\n",
    stats.TotalConns(), stats.IdleConns(), stats.AcquiredConns())
```

### ✅ Batch Operations (Unique to pgx!)

Execute multiple queries in a single round trip:

```go
import "github.com/stephenafamo/bob/drivers/pgx"

// Create batch
batch := pgx.NewQueuedBatch()

// Queue multiple operations
var users []models.User
for _, name := range names {
    var user models.User
    insertQ := models.Users.Insert(&models.UserSetter{Name: omit.From(name)})
    pgx.QueueInsertRowReturning(batch, ctx, insertQ,
        scan.StructMapper[models.User](), &user)
    users = append(users, user)
}

// Execute all in ONE round trip
batch.Execute(ctx, pool)

// All users now populated with IDs from database
```

See [BATCH_USAGE.md](./BATCH_USAGE.md) for complete batch documentation.

### ✅ Transactions

```go
tx, _ := pool.Begin(ctx)
defer tx.Rollback(ctx)

models.Users.Insert(setter).One(ctx, tx)
models.Posts.Insert(postSetter).One(ctx, tx)

tx.Commit(ctx)
```

### ✅ Acquire Individual Connections

```go
// Manual acquire/release
conn, _ := pool.Acquire(ctx)
defer conn.Release()

user, _ := models.Users.Query().One(ctx, conn)

// Or use AcquireFunc (auto-releases)
pool.AcquireFunc(ctx, func(conn pgx.PoolConn) error {
    return models.Users.Insert(setter).Exec(ctx, conn)
})
```

### ✅ PostgreSQL-Specific Types

Full support for PostgreSQL types:
- Arrays: `[]string`, `[]int`, etc.
- JSON/JSONB: `json.RawMessage`, `map[string]any`
- UUID: `github.com/google/uuid`
- Ranges, Geometric types, Network types, etc.

## Connection Types

This package provides three wrapper types:

### 1. `pgx.Pool` - Connection Pool (Primary)

Wraps `pgxpool.Pool` for pooled connections:

```go
pool, _ := pgx.New(ctx, dsn)
defer pool.Close()
```

### 2. `pgx.PoolConn` - Single Connection from Pool

Acquired from pool for specific operations:

```go
conn, _ := pool.Acquire(ctx)
defer conn.Release()
```

### 3. `pgx.Conn` - Direct Connection (Rare)

For non-pooled connections:

```go
pgxConn, _ := pgx.Connect(ctx, dsn)
conn := pgx.NewConn(pgxConn)
defer conn.Close(ctx)
```

**Recommendation**: Use `pgx.Pool` for 99% of use cases.

## Environment Variables

```bash
# Standard PostgreSQL environment variables work
export PGHOST=localhost
export PGPORT=5432
export PGDATABASE=mydb
export PGUSER=user
export PGPASSWORD=password

# Then connect with empty DSN
pool, _ := pgx.New(ctx, "")
```

## Connection String Formats

```go
// Standard PostgreSQL URL
"postgres://user:pass@localhost:5432/dbname"

// With SSL
"postgres://user:pass@localhost:5432/dbname?sslmode=require"

// With connection pool params
"postgres://user:pass@localhost:5432/dbname?pool_max_conns=10"

// Keyword/value format
"host=localhost port=5432 dbname=mydb user=user password=pass"
```

## Best Practices

### ✅ DO

```go
// Create pool once, reuse throughout application
var pool *pgx.Pool

func init() {
    pool, _ = pgx.New(context.Background(), dsn)
}

// Close pool on shutdown
defer pool.Close()

// Use batch for multiple operations
batch := pgx.NewQueuedBatch()
// ... queue operations
batch.Execute(ctx, pool)
```

### ❌ DON'T

```go
// Don't create new pool for each request
func handler() {
    pool, _ := pgx.New(ctx, dsn) // WRONG!
    defer pool.Close()
}

// Don't use other PostgreSQL drivers
import _ "github.com/lib/pq" // WRONG! Use pgx native

// Don't skip connection pooling
conn, _ := pgx.Connect(ctx, dsn) // Usually wrong, use Pool
```

## Production Configuration Example

```go
package database

import (
    "context"
    "time"
    "github.com/stephenafamo/bob/drivers/pgx"
    "github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(dsn string) (*pgx.Pool, error) {
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }

    // Production settings
    config.MaxConns = 25                              // Max connections
    config.MinConns = 5                               // Min idle connections
    config.MaxConnLifetime = time.Hour                // Recycle connections
    config.MaxConnIdleTime = 30 * time.Minute        // Close idle connections
    config.HealthCheckPeriod = time.Minute           // Check connection health
    config.ConnConfig.ConnectTimeout = 5 * time.Second // Connection timeout

    ctx := context.Background()
    return pgx.NewWithConfig(ctx, config)
}
```

## Monitoring

```go
// Pool statistics
stats := pool.Stat()
log.Printf("Pool Stats: Total=%d Idle=%d Acquired=%d",
    stats.TotalConns(),
    stats.IdleConns(),
    stats.AcquiredConns())

// Wait duration
log.Printf("Max wait duration: %v", stats.MaxLifetimeDestroyCount())
```

## Migration from Other Drivers

### From `lib/pq`

```go
// OLD (lib/pq)
import (
    "database/sql"
    _ "github.com/lib/pq"
)

db, _ := sql.Open("postgres", dsn)
bobDB := bob.NewDB(db)

// NEW (pgx native)
import "github.com/stephenafamo/bob/drivers/pgx"

pool, _ := pgx.New(ctx, dsn)
// pool is already a bob.Executor, no wrapping needed!
```

### From `pgx/stdlib`

```go
// OLD (pgx/stdlib)
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

db, _ := sql.Open("pgx", dsn)
bobDB := bob.NewDB(db)

// NEW (pgx native)
import "github.com/stephenafamo/bob/drivers/pgx"

pool, _ := pgx.New(ctx, dsn)
// Get batch support and better performance!
```

## FAQ

**Q: Can I use `lib/pq` or `pgx/stdlib` instead?**
A: No. Use pgx native. It's faster, has more features, and supports batch operations.

**Q: Do I need `database/sql`?**
A: No. pgx native doesn't use `database/sql`. That's why it's faster!

**Q: What about migrations?**
A: Use migration tools that support pgx directly (e.g., `goose`, `golang-migrate` with pgx driver).

**Q: How do I mock for tests?**
A: Mock `bob.Executor` interface. See Bob testing documentation.

## Documentation

- [Batch Operations](./BATCH_USAGE.md) - Complete batch API guide
- [Batch with Models](./BATCH_WITH_MODELS.md) - Using batches with generated models
- [pgx Documentation](https://pkg.go.dev/github.com/jackc/pgx/v5) - Official pgx docs

## Support

- Bob Issues: https://github.com/stephenafamo/bob/issues
- pgx Issues: https://github.com/jackc/pgx/issues

## License

Same as Bob ORM - see repository root LICENSE file.

---

**Remember**: If you're using PostgreSQL with Bob, use this driver. It's not just recommended—it's the right choice. 🎯
