---
sidebar_position: 11
title: PostgreSQL Driver
description: ORM Generation for PostgreSQL
---

# Bob Gen for Postgres

Generates an ORM based on a postgres database schema

## Usage

```sh
# With env variable
PSQL_DSN=postgres://user:pass@host:port/dbname go run github.com/templatedop/bob/gen/dopgen-psql@latest

# With configuration file
go run github.com/templatedop/bob/gen/dopgen-psql@latest -c ./config/bobgen.yaml
```

### Driver Configuration

#### [Link to general configuration and usage](./configuration)

The configuration for the postgres driver must all be prefixed by the driver name. You must use a configuration file or environment variables for configuring the database driver.

In the configuration file for postgresql for example you would do:

```yaml
psql:
  dsn: "postgres://user:pass@host:port/dbname"
```

When you use an environment variable it must also be prefixed by the driver name:

```sh
PSQL_DSN="postgres://user:pass@host:port/dbname"
```

Additionally if ssl mode is to be disabled (you will get connection failed error - `unable to fetch table data: unable to load enums: pq: SSL is not enabled on the server`), you can add `sslmode` to the dsn:

```sh
PSQL_DSN="postgres://user:pass@host:port/dbname?sslmode=disable"
```

The values that exist for the drivers:

| Name          | Description                                       | Default                  |
| ------------- | ------------------------------------------------- | ------------------------ |
| driver        | Driver to use for generating driver-specific code | `github.com/lib/pq`      |
| dsn           | URL to connect to                                 |                          |
| schemas       | Schemas find tables in                            | ["public"]               |
| shared_schema | Schema to not include prefix in model             | first value in "schemas" |
| uuid_pkg      | UUID package to use (gofrs or google)             | "gofrs"                  |
| queries       | Folders containing sql query files                |                          |
| only          | Only generate these                               |                          |
| except        | Skip generation for these                         |                          |
| concurrency   | How many tables to fetch in parallel              | 10                       |

## Driver-specific code

The `driver` configuration option enables Bob to generate code that is tailored to the specifics of the selected `database/sql` driver.

For Postgres, the supported drivers are:

- [github.com/lib/pq](https://pkg.go.dev/github.com/lib/pq) (default)
- [github.com/jackc/pgx](https://pkg.go.dev/github.com/jackc/pgx)
- [github.com/jackc/pgx/v4](https://pkg.go.dev/github.com/jackc/pgx/v4)
- [github.com/jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5)

Bob leverages driver-specific code to perform precise error matching for [generated error constants](./usage#generated-error-constants).

## Only/Except:

The `only` and `except` configuration options can be used to specify which tables to include or exclude from code generation. You can either supply a list of table names or use regular expressions to match multiple tables.

Consider the example below:

```yaml
psql:
  only:
    "/^foo/":
    bar_baz:
```

This configuration only generates models for tables that start with `foo` and the table named `bar_baz`.

Alternatively, the following example excludes these tables from code generation rather than including them:

```yaml
psql:
  except:
    "/^foo/":
    bar_baz:
```

You may also exclude specific columns:

```yaml
psql:
  # Removes public.migrations table, the name column from the addresses table, and
  # secret_col of any table from being generated. Foreign keys that reference tables
  # or columns that are no longer generated may cause problems.
  except:
    public.migrations:
    public.addresses:
      - name
    "*":
      - secret_col
```

## Batch Operations

PostgreSQL with Bob supports high-performance batch operations that can provide **10-100x performance improvements** for bulk inserts, updates, and related operations.

### Quick Example

```go
// Acquire connection
conn, _ := pool.Acquire(ctx)
defer conn.Release()

// Create batch - NO explicit transaction needed!
batch := &pgx.Batch{}
for _, product := range products {
    batch.Queue(
        "INSERT INTO products (name, price) VALUES ($1, $2)",
        product.Name, product.Price,
    )
}

// Send batch - implicitly transactional
results := conn.SendBatch(ctx, batch)
defer results.Close()

// Process results
for range products {
    _, err := results.Exec()
    // handle error
}
```

**Performance:** Inserts 1000 products in ~50ms instead of ~5000ms

### Learn More

- **[Complete Batch Operations Guide](./batch-operations)** - Full documentation from YAML to production API
- **[Example Project](https://github.com/templatedop/bob/tree/main/examples/batch-api)** - Runnable REST API with batch operations
- **[Test Suite](https://github.com/templatedop/bob/tree/main/test/batch_codegen)** - Comprehensive test examples

### Key Features

✅ **Implicitly Transactional** - No need for explicit `BEGIN/COMMIT`
✅ **Multiple Operations** - Mix INSERT, SELECT, UPDATE, DELETE in one batch
✅ **Error Handling** - All operations succeed or all rollback
✅ **Works with Generated Code** - Use your dopgen-psql generated queries

See the [Batch Operations Guide](./batch-operations) for complete workflow from YAML configuration to production deployment.
