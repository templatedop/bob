-- name: GetProduct :one
SELECT id, name, price, stock, category_id, created_at, updated_at
FROM products
WHERE id = $1;
