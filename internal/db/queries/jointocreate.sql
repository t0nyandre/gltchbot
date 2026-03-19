-- name: CreateJTCParentChannel :one
INSERT INTO jtc_parent_channels (guild_id, channel_id, category_id, channel_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetJTCParentChannel :one
SELECT * FROM jtc_parent_channels WHERE channel_id = $1;

-- name: ListJTCParentChannels :many
SELECT * FROM jtc_parent_channels WHERE guild_id = $1;

-- name: CountJTCParentChannels :one
SELECT COUNT(*) FROM jtc_parent_channels WHERE guild_id = $1;

-- name: ListJTCParentChannelsPaginated :many
SELECT * FROM jtc_parent_channels WHERE guild_id = $1 LIMIT $2 OFFSET $3;

-- name: DeleteJTCParentChannel :exec
DELETE FROM jtc_parent_channels WHERE channel_id = $1 AND guild_id = $2;

-- name: CreateJTCActiveChannel :one
INSERT INTO jtc_active_channels (channel_id, guild_id, owner_id, parent_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetJTCActiveChannel :one
SELECT * FROM jtc_active_channels WHERE channel_id = $1;

-- name: DeleteJTCActiveChannel :exec
DELETE FROM jtc_active_channels WHERE channel_id = $1;

-- name: ListJTCActiveChannels :many
SELECT * FROM jtc_active_channels WHERE guild_id = $1;

-- name: UpsertJTCUserSettings :exec
INSERT INTO jtc_user_settings (guild_id, user_id, custom_name)
VALUES ($1, $2, $3)
ON CONFLICT (guild_id, user_id) DO UPDATE SET custom_name = EXCLUDED.custom_name;

-- name: GetJTCUserSettings :one
SELECT * FROM jtc_user_settings WHERE guild_id = $1 AND user_id = $2;
