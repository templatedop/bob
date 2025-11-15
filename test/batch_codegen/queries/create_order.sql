-- name: CreateOrder :exec
INSERT INTO orders (product_id, quantity, total_price)
VALUES ($1, $2, $3);
