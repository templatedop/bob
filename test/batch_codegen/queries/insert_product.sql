-- name: InsertProduct :exec
INSERT INTO products (name, price, stock)
VALUES ($1, $2, $3);
