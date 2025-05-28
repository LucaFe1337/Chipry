-- name: CreateChirps :one
INSERT INTO chirps(body, user_id)
VALUES(
	$1,
    $2
)
RETURNING *;