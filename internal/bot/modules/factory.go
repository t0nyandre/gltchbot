package modules

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/bot/modules/autorole"
	"github.com/t0nyandre/gltchbot/internal/bot/modules/jointocreate"
	"github.com/t0nyandre/gltchbot/internal/bot/modules/reactionroles"
)

// DefaultModules returns all available modules configured with the given database pool.
func DefaultModules(db *pgxpool.Pool) []Module {
	return []Module{
		jointocreate.New(db),
		reactionroles.New(db),
		autorole.New(db),
	}
}