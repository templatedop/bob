-- name: ListProducts :many
SELECT id, name, price, stock, category_id, created_at, updated_at
FROM products
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
