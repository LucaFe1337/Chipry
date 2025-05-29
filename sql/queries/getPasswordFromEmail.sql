-- name: GetPasswordFromEmail :one
SELECT * FROM users WHERE email = $1;