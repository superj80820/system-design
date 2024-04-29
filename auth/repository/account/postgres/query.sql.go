package postgres

import (
	"context"
	"database/sql"
)

const getAll = `-- name: GetAll :many
SELECT id, email, password, created_at, platform, updated_at FROM account ORDER BY created_at
`

func (q *Queries) GetAll(ctx context.Context) ([]Account, error) {
	rows, err := q.db.QueryContext(ctx, getAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Account
	for rows.Next() {
		var i Account
		if err := rows.Scan(
			&i.ID,
			&i.Email,
			&i.Password,
			&i.CreatedAt,
			&i.Platform,
			&i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateID = `-- name: UpdateID :exec
UPDATE account SET id = $1 WHERE id = $2
`

type UpdateIDParams struct {
	ID   string
	ID_2 string
}

func (q *Queries) UpdateID(ctx context.Context, arg UpdateIDParams) error {
	_, err := q.db.ExecContext(ctx, updateID, arg.ID, arg.ID_2)
	return err
}

const updateUpdatedTime = `-- name: UpdateUpdatedTime :exec
UPDATE account SET updated_at = $1 WHERE id = $2
`

type UpdateUpdatedTimeParams struct {
	UpdatedAt sql.NullTime
	ID        string
}

func (q *Queries) UpdateUpdatedTime(ctx context.Context, arg UpdateUpdatedTimeParams) error {
	_, err := q.db.ExecContext(ctx, updateUpdatedTime, arg.UpdatedAt, arg.ID)
	return err
}
