-- Track which users have already triggered first_message/first_reaction per guild
-- This prevents assigning auto-roles multiple times for the same trigger

CREATE TABLE IF NOT EXISTS autorole_user_triggers (
    guild_id     TEXT NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL,
    -- Trigger type: 'first_message' | 'first_reaction'
    trigger      TEXT NOT NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id, trigger)
);

-- Index for faster lookups by guild and user
CREATE INDEX IF NOT EXISTS idx_autorole_user_triggers_guild_user 
    ON autorole_user_triggers (guild_id, user_id);

-- Index for faster lookups by guild and trigger
CREATE INDEX IF NOT EXISTS idx_autorole_user_triggers_guild_trigger 
    ON autorole_user_triggers (guild_id, trigger);