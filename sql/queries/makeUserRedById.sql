-- name: UpdateUserToRed :exec
UPDATE users SET is_chirpy_red = true WHERE id = $1;