-- name: GetProduct :one
SELECT id, name, price, stock, created_at
FROM products
WHERE id = $1;
