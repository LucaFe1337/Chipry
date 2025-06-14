// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.29.0
// source: updateUserData.sql

package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const updateUserData = `-- name: UpdateUserData :exec
UPDATE users SET email = $2, hashed_password = $3, updated_at = $4 WHERE id = $1
`

type UpdateUserDataParams struct {
	ID             uuid.UUID
	Email          string
	HashedPassword string
	UpdatedAt      time.Time
}

func (q *Queries) UpdateUserData(ctx context.Context, arg UpdateUserDataParams) error {
	_, err := q.db.ExecContext(ctx, updateUserData,
		arg.ID,
		arg.Email,
		arg.HashedPassword,
		arg.UpdatedAt,
	)
	return err
}
