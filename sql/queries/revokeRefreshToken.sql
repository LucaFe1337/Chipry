-- name: RevokeRefreshToken :exec
UPDATE refresh_token SET revoked_at = $1, updated_at = $1 WHERE token = $2;