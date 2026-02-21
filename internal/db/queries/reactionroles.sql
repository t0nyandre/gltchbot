-- name: CreateReactionRole :one
INSERT INTO reaction_roles (guild_id, channel_id, message_id, emoji, role_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetReactionRole :one
SELECT * FROM reaction_roles WHERE id = $1;

-- name: GetReactionRoleByMessageAndEmoji :one
SELECT * FROM reaction_roles WHERE message_id = $1 AND emoji = $2;

-- name: ListReactionRolesByGuild :many
SELECT * FROM reaction_roles 
WHERE guild_id = $1 
ORDER BY channel_id, message_id;

-- name: ListReactionRolesByMessage :many
SELECT * FROM reaction_roles WHERE message_id = $1;

-- name: DeleteReactionRole :exec
DELETE FROM reaction_roles WHERE id = $1;

-- name: DeleteReactionRoleByMessageAndEmoji :exec
DELETE FROM reaction_roles WHERE message_id = $1 AND emoji = $2;

-- name: DeleteAllReactionRolesForMessage :exec
DELETE FROM reaction_roles WHERE message_id = $1;

-- name: DeleteAllReactionRolesForGuild :exec
DELETE FROM reaction_roles WHERE guild_id = $1;