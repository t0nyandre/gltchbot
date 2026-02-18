# GltchBot Discord Bot

A modular, extensible Discord bot built with Go and discordgo, backed by PostgreSQL.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Bot | Go + [discordgo](https://github.com/bwmarrin/discordgo) |
| API | Go `net/http` (stdlib, Go 1.22+) |
| Database | PostgreSQL 17 |
| DB Queries | [sqlc](https://sqlc.dev) + [pgx/v5](https://github.com/jackc/pgx) |
| Migrations | [golang-migrate](https://github.com/golang-migrate/migrate) |
| Containers | Docker + Docker Compose |

## Project Structure

```
.
├── cmd/
│   ├── bot/main.go          # Bot entry point
│   └── api/main.go          # API entry point
├── internal/
│   ├── bot/
│   │   ├── client.go        # Discord session + startup
│   │   └── modules/         # Feature modules
│   │       ├── module.go    # Module interface
│   │       ├── registry.go  # Module loader & lifecycle
│   │       ├── jointocreate/
│   │       ├── reactionroles/
│   │       └── autorole/
│   ├── api/
│   │   ├── server.go        # HTTP server setup
│   │   ├── middleware/      # API key auth
│   │   └── routes/          # Route handlers
│   ├── config/              # Environment config
│   └── db/
│       ├── db.go            # Connection pool + migrations
│       ├── migrations/      # SQL migration files
│       ├── queries/         # SQL query files (sqlc input)
│       └── sqlc/            # Generated Go code (do not edit)
├── docker-compose.yml
├── Dockerfile.bot
├── Dockerfile.api
├── sqlc.yaml
└── Makefile
```

## Getting Started

### 1. Configure Environment

```bash
cp .env.example .env
# Edit .env with your Discord token, app ID, DB password, and API key
```

### 2. Run with Docker Compose

```bash
make docker-up
# or
docker compose up -d --build
```

This starts:
- **PostgreSQL** on port `5432`
- **Bot** (connects to Discord)
- **API** on port `8080`

### 3. Run Locally (Development)

```bash
# Start just PostgreSQL
docker compose up -d postgres

# Run the bot (in one terminal)
make run-bot

# Run the API (in another terminal)
make run-api
```

## Module System

Each module implements the `Module` interface:

```go
type Module interface {
    Name() string
    Description() string
    Commands() []*discordgo.ApplicationCommand
    RegisterHandlers(s *discordgo.Session)
    OnEnable(ctx context.Context, db *pgxpool.Pool, guildID string) error
    OnDisable(ctx context.Context, db *pgxpool.Pool, guildID string) error
}
```

Modules are enabled/disabled per guild via the API:

```bash
# Enable JoinToCreate for a guild
curl -X PATCH http://localhost:8080/api/guilds/{guildId}/modules/jointocreate \
  -H "X-API-Key: your_api_key" \
  -H "Content-Type: application/json" \
  -d '{"enabled": true}'
```

## Implemented Modules

### ✅ JoinToCreate

Creates temporary voice channels when a user joins a parent channel.

**Setup:** `/jointocreate setup category:<category> channel_name:<name>`

**Features:**
- Auto-creates a cloned voice channel named after the joining user
- Naming priority: Server Nickname → Global Display Name → Username
- Auto-deletes the channel when it becomes empty
- Persists user's custom channel name preferences across sessions

**Commands:**
- `/jointocreate setup` — Set up a parent channel
- `/jointocreate remove` — Remove a parent channel
- `/jointocreate list` — List all parent channels

### 🔲 Reaction Roles *(scaffolded)*

Assign roles to users who react to a specific message. Coming soon.

### 🔲 Auto Role *(scaffolded)*

Automatically assign roles on join or first activity. Coming soon.

## API Reference

All API endpoints (except `/health`) require the `X-API-Key` header.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check (no auth) |
| `GET` | `/api/guilds` | List all guilds |
| `GET` | `/api/guilds/{guildId}` | Get a guild |
| `GET` | `/api/guilds/{guildId}/modules` | List modules + enabled status |
| `GET` | `/api/guilds/{guildId}/modules/{moduleName}` | Get module status |
| `PATCH` | `/api/guilds/{guildId}/modules/{moduleName}` | Enable/disable a module |
| `GET` | `/api/guilds/{guildId}/modules/jointocreate` | Get JTC config |
| `POST` | `/api/guilds/{guildId}/modules/jointocreate/parents` | Add parent channel |
| `DELETE` | `/api/guilds/{guildId}/modules/jointocreate/parents/{channelId}` | Remove parent channel |

## Database Migrations

Migrations run automatically on startup. To regenerate sqlc code after modifying queries:

```bash
make sqlc
```

## Adding a New Module

1. Create `internal/bot/modules/yourmodule/module.go` implementing the `Module` interface
2. Add SQL migrations in `internal/db/migrations/`
3. Add SQL queries in `internal/db/queries/`
4. Run `make sqlc` to regenerate DB code
5. Register the module in `internal/bot/client.go`:
   ```go
   registry.Register(yourmodule.New(db))
   ```
6. Seed the module name in `000001_init.up.sql` `INSERT INTO modules`
