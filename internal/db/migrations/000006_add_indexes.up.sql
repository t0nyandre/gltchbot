-- Add performance indexes for frequently queried columns and foreign keys

-- Indexes for jtc_parent_channels
CREATE INDEX IF NOT EXISTS idx_jtc_parent_channels_guild_id ON jtc_parent_channels (guild_id);

-- Indexes for jtc_active_channels
CREATE INDEX IF NOT EXISTS idx_jtc_active_channels_guild_id ON jtc_active_channels (guild_id);
CREATE INDEX IF NOT EXISTS idx_jtc_active_channels_parent_id ON jtc_active_channels (parent_id);
-- owner_id is currently not queried, but index for future use
CREATE INDEX IF NOT EXISTS idx_jtc_active_channels_owner_id ON jtc_active_channels (owner_id);

-- Indexes for reaction_roles (foreign key guild_id and query patterns)
CREATE INDEX IF NOT EXISTS idx_reaction_roles_guild_id ON reaction_roles (guild_id);
-- message_id already indexed via UNIQUE (message_id, emoji), but we add a separate index for guild_id+message_id queries if needed
-- Not needed now.

-- Indexes for auto_roles (for guild_id and trigger combinations)
CREATE INDEX IF NOT EXISTS idx_auto_roles_guild_id_trigger ON auto_roles (guild_id, trigger);
-- guild_id already covered by UNIQUE (guild_id, role_id, trigger) but composite index helps guild_id+trigger queries

-- Indexes for foreign keys that are missing indexes (cascading deletes)
-- guild_modules already has PRIMARY KEY (guild_id, module_id)
-- autorole_user_triggers already has indexes
-- No additional indexes needed for foreign keys beyond those above.