-- name: LogInventory :exec
INSERT INTO inventory_log (product_id, change_amount, reason)
VALUES ($1, $2, $3);
