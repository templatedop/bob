-- name: UpdateStock :exec
UPDATE products
SET stock = stock + $2
WHERE id = $1;
