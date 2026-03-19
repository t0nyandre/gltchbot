-- name: UpsertGuild :one
INSERT INTO guilds (id, name)
VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name
RETURNING *;

-- name: GetGuild :one
SELECT * FROM guilds WHERE id = $1;

-- name: ListGuilds :many
SELECT * FROM guilds ORDER BY created_at DESC;

-- name: CountGuilds :one
SELECT COUNT(*) FROM guilds;

-- name: ListGuildsPaginated :many
SELECT * FROM guilds ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: DeleteGuild :exec
DELETE FROM guilds WHERE id = $1;
