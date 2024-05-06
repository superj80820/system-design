-- name: GetActressesByPagination :many
SELECT * FROM actresses OFFSET $1 LIMIT $2;

-- name: GetActressSize :one
SELECT COUNT(*) FROM actresses;

-- name: GetActressesByIDs :many
SELECT * FROM actresses WHERE id = ANY($1::int[]) ORDER BY id DESC;