-- AutoRoles table queries

-- name: CreateAutoRole :one
INSERT INTO auto_roles (guild_id, role_id, trigger)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAutoRole :one
SELECT * FROM auto_roles WHERE id = $1;

-- name: GetAutoRoleByGuildAndRoleAndTrigger :one
SELECT * FROM auto_roles WHERE guild_id = $1 AND role_id = $2 AND trigger = $3;

-- name: ListAutoRolesByGuild :many
SELECT * FROM auto_roles WHERE guild_id = $1 ORDER BY trigger, role_id;

-- name: ListAutoRolesByGuildAndTrigger :many
SELECT * FROM auto_roles WHERE guild_id = $1 AND trigger = $2 ORDER BY role_id;

-- name: DeleteAutoRole :exec
DELETE FROM auto_roles WHERE id = $1;

-- name: DeleteAutoRoleByGuildAndRoleAndTrigger :exec
DELETE FROM auto_roles WHERE guild_id = $1 AND role_id = $2 AND trigger = $3;

-- name: DeleteAllAutoRolesForGuild :exec
DELETE FROM auto_roles WHERE guild_id = $1;


-- AutoroleUserTriggers table queries

-- name: CreateUserTrigger :exec
INSERT INTO autorole_user_triggers (guild_id, user_id, trigger)
VALUES ($1, $2, $3)
ON CONFLICT (guild_id, user_id, trigger) DO NOTHING;

-- name: GetUserTrigger :one
SELECT * FROM autorole_user_triggers WHERE guild_id = $1 AND user_id = $2 AND trigger = $3;

-- name: DeleteUserTriggersForGuild :exec
DELETE FROM autorole_user_triggers WHERE guild_id = $1;