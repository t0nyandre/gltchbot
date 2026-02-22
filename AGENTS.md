# Agent Guidelines for Vibe-Code Discord Bot

## Project Overview
Go 1.25 Discord bot with PostgreSQL backend, modular architecture. Two binaries:
- **Bot**: Main Discord bot handling slash commands and events
- **API**: REST API for administrative tasks and dashboard

**Key Architecture:** Module registry system (`internal/bot/modules/`), sqlc for type-safe SQL queries, embedded migrations.

## Development Setup
**Prerequisites:** Go 1.25+, PostgreSQL 15+, Docker (optional), sqlc (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
**Environment:** `cp .env.example .env; cp .env.dev.example .env.dev`

## Build Commands
```bash
make build          # Build both binaries
make build-bot      # Build bot only  
make build-api      # Build API only
make run-bot        # Run bot locally
make run-api        # Run API locally
```

## Lint & Test Commands
**Code Quality:**
```bash
make vet            # go vet
make tidy           # Tidy go.mod and go.sum
gofmt -w .          # Format all Go files
gofmt -l .          # List unformatted files
```

**Testing:**
```bash
go test ./...              # Run all tests
go test ./internal/config  # Specific package
go test -v ./...           # Verbose output
go test -run TestGetEnvOrDefault    # Specific test by name
go test -count=1 ./...     # Disable test caching
```

**Single Test:** `go test -run TestGetEnvOrDefault ./internal/config`

## Database Operations
**SQLc Generation:** `make sqlc`
**Migrations:** Embedded via `embed.FS` in `internal/db/migrations/`. Number sequentially, include `.up.sql` and `.down.sql`.
**Local Database:**
```bash
docker compose up -d  # Production
# Dev (wipes data on down):
docker compose --env-file .env.dev -f docker-compose.dev.yml up -d --build
```

## Docker Commands
```bash
make docker-up          # Start production containers
make docker-down        # Stop production containers
make docker-logs        # Follow logs
make docker-dev-up      # Start dev containers (wipes volumes on down)
make docker-dev-down    # Stop dev containers and remove volumes
make docker-dev-logs    # Follow dev logs
```

## Code Style Guidelines (Go 1.25+)
**General Principles:**
- No global state: Dependency injection via constructors
- Explicit over implicit: Clear initialization patterns
- Error handling: Return `error`, not panic
- Context propagation: Pass `context.Context` for cancellation/timeout

**Module Development Pattern:**
1. Single responsibility: Each module handles one feature
2. Self-contained: Modules register own commands/handlers
3. Database-aware: Use `OnEnable`/`OnDisable` for guild lifecycle
4. Stateless: Store guild-specific config in database, not memory

**Go-Specific:**
- Use goimports for import organization
- Avoid `init()`: Use explicit initialization functions
- Error wrapping: `fmt.Errorf("...: %w", err)` for context
- SQLc integration: Queries in `internal/db/queries/`, generated code in `internal/db/sqlc/`

## CI/CD Workflow
**GitHub Actions:**
1. **CI (`ci.yml`)**: Runs on PRs to `main` - checks go.mod tidiness, validates formatting, runs `go vet` and `go test ./...`
2. **Deploy (`deploy.yml`)**: Manual trigger - builds binaries with `-ldflags="-s -w"`, deploys to VPS via SSH

**Local CI Simulation:**
```bash
go mod tidy && git diff --exit-code go.mod go.sum
unformatted=$(gofmt -l .); [ -z "$unformatted" ] || exit 1
go vet ./...
go test ./...
```

**Workflow Improvements:** Add caching, matrix testing, database testing, security scanning.

## Contributing New Bot Modules
**Module Interface:** Implement `modules.Module` from `internal/bot/modules/module.go`:
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

**Module Structure:** `internal/bot/modules/<modulename>/` with `module.go`, `handlers.go`, `commands.go`, `<modulename>_test.go`
**Registration:** `registry.Register(autorole.New(db))` in `cmd/bot/main.go`
**Database:** Use `modules` table for metadata, `guild_modules` for guild-specific state, store config as JSONB.
**Testing:** Unit tests for module logic, integration tests for database interactions, mock Discord utilities.

## Agent Guidelines
**Before Making Changes:**
1. Check existing tests: `go test ./...`
2. Verify formatting: `gofmt -l .`
3. Update dependencies: `go mod tidy` if adding imports

**When Adding Features:**
1. Follow module pattern: New directory in `internal/bot/modules/`
2. Add database migrations if schema changes needed
3. Update sqlc queries in `internal/db/queries/`
4. Write unit and integration tests
5. Document complex logic with comments

**Code Review Checklist:**
- [ ] Go modules tidy (`go mod tidy`)
- [ ] Code formatted (`gofmt`)
- [ ] No vet errors (`go vet`)
- [ ] Tests pass (`go test ./...`)
- [ ] Database migrations included if needed
- [ ] SQLc queries updated if needed
- [ ] Module follows interface pattern

**Common Pitfalls:**
- Missing error handling: Always check `err` returns
- Race conditions: Use mutex for shared state in modules
- Database leaks: Close rows and connections properly
- Discord rate limits: Implement proper backoff in handlers

**Performance Considerations:**
- Database pooling: Use `pgxpool.Pool` for connections
- Query optimization: Use EXPLAIN ANALYZE for slow queries
- Memory profiling: Use `pprof` for memory leaks
- Goroutine management: Use wait groups for cleanup

---

*Last Updated: 2025-02-22*  
*Go Version: 1.25+*  
*Database: PostgreSQL 15+*