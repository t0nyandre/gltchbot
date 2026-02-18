-- JoinToCreate module tables

-- Parent channels that trigger the JoinToCreate behaviour
CREATE TABLE IF NOT EXISTS jtc_parent_channels (
    id           SERIAL PRIMARY KEY,
    guild_id     TEXT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    channel_id   TEXT UNIQUE NOT NULL,
    category_id  TEXT NOT NULL,
    channel_name TEXT NOT NULL
);

-- Temporary channels that are currently active (created by JoinToCreate)
CREATE TABLE IF NOT EXISTS jtc_active_channels (
    channel_id TEXT PRIMARY KEY,
    guild_id   TEXT NOT NULL,
    owner_id   TEXT NOT NULL,
    parent_id  TEXT NOT NULL REFERENCES jtc_parent_channels(channel_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Persisted per-user channel preferences (name saved across sessions)
CREATE TABLE IF NOT EXISTS jtc_user_settings (
    guild_id    TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    custom_name TEXT,
    PRIMARY KEY (guild_id, user_id)
);
