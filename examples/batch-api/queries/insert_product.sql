-- name: InsertProduct :one
INSERT INTO products (name, price, stock, category_id)
VALUES ($1, $2, $3, $4)
RETURNING id, name, price, stock, category_id, created_at, updated_at;
