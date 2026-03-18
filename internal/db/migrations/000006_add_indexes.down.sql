-- Drop performance indexes added in the up migration

DROP INDEX IF EXISTS idx_jtc_parent_channels_guild_id;
DROP INDEX IF EXISTS idx_jtc_active_channels_guild_id;
DROP INDEX IF EXISTS idx_jtc_active_channels_parent_id;
DROP INDEX IF EXISTS idx_jtc_active_channels_owner_id;
DROP INDEX IF EXISTS idx_reaction_roles_guild_id;
DROP INDEX IF EXISTS idx_auto_roles_guild_id_trigger;