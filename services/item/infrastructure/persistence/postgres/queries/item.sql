-- name: InsertItem :exec
INSERT INTO item.items (id, org_id, name, created_at)
VALUES ($1, $2, $3, $4);

-- name: GetItemByID :one
SELECT id, org_id, name, created_at
FROM item.items
WHERE id = $1 AND org_id = $2;

-- name: FindItemsByOrgID :many
SELECT id, org_id, name, created_at
FROM item.items
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountItemsByOrgID :one
SELECT COUNT(*) FROM item.items
WHERE org_id = $1;

-- name: UpdateItem :exec
UPDATE item.items
SET name = $1
WHERE id = $2 AND org_id = $3;

-- name: DeleteItem :exec
DELETE FROM item.items
WHERE id = $1 AND org_id = $2;

-- name: ItemExists :one
SELECT EXISTS(
    SELECT 1 FROM item.items
    WHERE id = $1 AND org_id = $2
);
