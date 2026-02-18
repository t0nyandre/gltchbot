-- name: ListModules :many
SELECT * FROM modules ORDER BY name;

-- name: GetModuleByName :one
SELECT * FROM modules WHERE name = $1;

-- name: ListGuildModules :many
SELECT
    m.id,
    m.name,
    m.description,
    COALESCE(gm.enabled, false) AS enabled,
    COALESCE(gm.config, '{}')   AS config
FROM modules m
LEFT JOIN guild_modules gm ON gm.module_id = m.id AND gm.guild_id = $1
ORDER BY m.name;

-- name: GetGuildModule :one
SELECT
    m.id,
    m.name,
    m.description,
    COALESCE(gm.enabled, false) AS enabled,
    COALESCE(gm.config, '{}')   AS config
FROM modules m
LEFT JOIN guild_modules gm ON gm.module_id = m.id AND gm.guild_id = $1
WHERE m.name = $2;

-- name: UpsertGuildModule :exec
INSERT INTO guild_modules (guild_id, module_id, enabled, config)
VALUES ($1, $2, $3, $4)
ON CONFLICT (guild_id, module_id) DO UPDATE
    SET enabled = EXCLUDED.enabled,
        config  = EXCLUDED.config;

-- name: IsModuleEnabled :one
SELECT COALESCE(gm.enabled, false) AS enabled
FROM modules m
LEFT JOIN guild_modules gm ON gm.module_id = m.id AND gm.guild_id = $1
WHERE m.name = $2;
