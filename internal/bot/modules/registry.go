package modules

import (
	"context"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	dbsqlc "github.com/t0nyandre/gltchbot/internal/db/sqlc"
)

// Registry manages all registered modules and their lifecycle.
type Registry struct {
	modules map[string]Module
	db      *pgxpool.Pool
	queries *dbsqlc.Queries
}

// NewRegistry creates an empty module registry.
func NewRegistry(db *pgxpool.Pool) *Registry {
	return &Registry{
		modules: make(map[string]Module),
		db:      db,
		queries: dbsqlc.New(db),
	}
}

// Register adds a module to the registry.
func (r *Registry) Register(m Module) {
	r.modules[m.Name()] = m
	log.Printf("module registered: %s", m.Name())
}

// Get returns a module by name.
func (r *Registry) Get(name string) (Module, bool) {
	m, ok := r.modules[name]
	return m, ok
}

// All returns all registered modules.
func (r *Registry) All() []Module {
	out := make([]Module, 0, len(r.modules))
	for _, m := range r.modules {
		out = append(out, m)
	}
	return out
}

// RegisterHandlers attaches event handlers for all registered modules.
// Each module registers its own handlers — no guild-filtering needed here
// because the handlers themselves check DB state per event.
func (r *Registry) RegisterHandlers(s *discordgo.Session) {
	for _, m := range r.modules {
		m.RegisterHandlers(s)
		log.Printf("handlers registered for module: %s", m.Name())
	}
}

// RegisterCommands registers slash commands either globally or per-guild
// depending on whether devGuildID is set.
func (r *Registry) RegisterCommands(s *discordgo.Session, appID, devGuildID string) error {
	// Collect all commands from all modules
	var allCommands []*discordgo.ApplicationCommand
	for _, m := range r.modules {
		allCommands = append(allCommands, m.Commands()...)
	}

	if len(allCommands) == 0 {
		log.Println("no slash commands to register")
		return nil
	}

	if devGuildID != "" {
		log.Printf("registering %d command(s) to dev guild %s (instant)", len(allCommands), devGuildID)
	} else {
		log.Printf("registering %d command(s) globally (up to 1h propagation)", len(allCommands))
	}

	_, err := s.ApplicationCommandBulkOverwrite(appID, devGuildID, allCommands)
	if err != nil {
		return fmt.Errorf("register slash commands: %w", err)
	}

	log.Println("slash commands registered successfully")
	return nil
}

// EnableForGuild enables a module for a specific guild, calling OnEnable.
func (r *Registry) EnableForGuild(ctx context.Context, moduleName, guildID string) error {
	m, ok := r.modules[moduleName]
	if !ok {
		return fmt.Errorf("module %q not found", moduleName)
	}

	// Get module DB record
	mod, err := r.queries.GetModuleByName(ctx, moduleName)
	if err != nil {
		return fmt.Errorf("get module from db: %w", err)
	}

	// Upsert guild_modules with enabled=true
	if err := r.queries.UpsertGuildModule(ctx, dbsqlc.UpsertGuildModuleParams{
		GuildID:  guildID,
		ModuleID: mod.ID,
		Enabled:  true,
		Config:   []byte("{}"),
	}); err != nil {
		return fmt.Errorf("upsert guild module: %w", err)
	}

	// Call module-specific enable hook
	if err := m.OnEnable(ctx, r.db, guildID); err != nil {
		return fmt.Errorf("module %s OnEnable: %w", moduleName, err)
	}

	log.Printf("module %s enabled for guild %s", moduleName, guildID)
	return nil
}

// DisableForGuild disables a module for a specific guild, calling OnDisable.
func (r *Registry) DisableForGuild(ctx context.Context, moduleName, guildID string) error {
	m, ok := r.modules[moduleName]
	if !ok {
		return fmt.Errorf("module %q not found", moduleName)
	}

	mod, err := r.queries.GetModuleByName(ctx, moduleName)
	if err != nil {
		return fmt.Errorf("get module from db: %w", err)
	}

	if err := r.queries.UpsertGuildModule(ctx, dbsqlc.UpsertGuildModuleParams{
		GuildID:  guildID,
		ModuleID: mod.ID,
		Enabled:  false,
		Config:   []byte("{}"),
	}); err != nil {
		return fmt.Errorf("upsert guild module: %w", err)
	}

	if err := m.OnDisable(ctx, r.db, guildID); err != nil {
		return fmt.Errorf("module %s OnDisable: %w", moduleName, err)
	}

	log.Printf("module %s disabled for guild %s", moduleName, guildID)
	return nil
}

// IsEnabledForGuild checks whether a module is enabled for the given guild.
func (r *Registry) IsEnabledForGuild(ctx context.Context, moduleName, guildID string) (bool, error) {
	enabled, err := r.queries.IsModuleEnabled(ctx, dbsqlc.IsModuleEnabledParams{
		GuildID: guildID,
		Name:    moduleName,
	})
	if err != nil {
		return false, err
	}
	return enabled, nil
}
