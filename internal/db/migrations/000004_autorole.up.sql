-- Auto Role module tables

-- Each row defines a role to be automatically assigned based on a trigger
CREATE TABLE IF NOT EXISTS auto_roles (
    id       SERIAL PRIMARY KEY,
    guild_id TEXT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    role_id  TEXT NOT NULL,
    -- Trigger: 'join' | 'first_message' | 'first_reaction'
    trigger  TEXT NOT NULL DEFAULT 'join',
    UNIQUE (guild_id, role_id, trigger)
);
