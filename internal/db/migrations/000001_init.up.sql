-- Core tables for guild and module management

CREATE TABLE IF NOT EXISTS guilds (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS modules (
    id          SERIAL PRIMARY KEY,
    name        TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS guild_modules (
    guild_id  TEXT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    module_id INT  NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    enabled   BOOLEAN NOT NULL DEFAULT true,
    config    JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (guild_id, module_id)
);

-- Seed the known modules
INSERT INTO modules (name, description) VALUES
    ('jointocreate',   'Automatically create temporary voice channels when a user joins a designated parent channel'),
    ('reactionroles',  'Allow users to assign roles to themselves by reacting to a message'),
    ('autorole',       'Automatically assign roles to users on join or first activity')
ON CONFLICT (name) DO NOTHING;
