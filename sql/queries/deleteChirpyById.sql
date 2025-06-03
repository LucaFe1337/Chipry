-- name: DeleteChripyById :exec
DELETE FROM chirps WHERE id = $1;