-- name: GetActressesByPagination :many
SELECT * FROM actresses OFFSET $1 LIMIT $2;

-- name: GetActressSize :one
SELECT COUNT(*) FROM actresses;

-- name: GetActressesByIDs :many
SELECT * FROM actresses WHERE id = ANY($1::int[]) ORDER BY id DESC;

-- name: GetFavoritesByPagination :many
SELECT * FROM actresses WHERE id IN (SELECT actress_id FROM account_favorite_actresses WHERE account_id = $1) OFFSET $2 LIMIT $3;

-- name: GetFavoriteSize :one
SELECT COUNT(*) FROM actresses WHERE id IN (SELECT actress_id FROM account_favorite_actresses WHERE account_id = $1);