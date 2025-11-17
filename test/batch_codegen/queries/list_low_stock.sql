-- name: ListLowStock :many
SELECT id, name, price, stock
FROM products
WHERE stock < $1
ORDER BY stock ASC;
