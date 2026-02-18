-- Reaction Roles module tables

-- Each row maps an emoji reaction on a specific message to a Discord role
CREATE TABLE IF NOT EXISTS reaction_roles (
    id         SERIAL PRIMARY KEY,
    guild_id   TEXT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    channel_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    emoji      TEXT NOT NULL,  -- Unicode emoji or custom emoji ID
    role_id    TEXT NOT NULL,
    UNIQUE (message_id, emoji)
);
