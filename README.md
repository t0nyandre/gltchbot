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

#### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DISCORD_TOKEN` | **required** | Discord bot token |
| `DISCORD_APP_ID` | **required** | Discord application ID |
| `DISCORD_DEV_GUILD_ID` | empty | Developer guild ID for guild-specific commands (optional) |
| `GO_ENV` | `development` | Runtime environment: `production`, `development`, `staging`. Affects security warnings and logging format. |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `gltchbot` | PostgreSQL user |
| `DB_PASSWORD` | **required** | PostgreSQL password |
| `DB_NAME` | `gltchbot` | Database name |
| `DB_SSLMODE` | `disable` | PostgreSQL SSL mode |
| `DB_MAX_CONNS` | `10` | Maximum number of database connections in pool |
| `DB_MIN_CONNS` | `2` | Minimum number of idle connections in pool |
| `DB_MAX_CONN_LIFETIME` | `1h` | Maximum lifetime of a connection |
| `DB_MAX_CONN_IDLE_TIME` | `30m` | Maximum idle time of a connection |
| `API_PORT` | `8080` | HTTP API port |
| `API_KEY` | **required** | API key(s) for securing endpoints (comma-separated, minimum 32 characters, high-entropy random strings). Generate with `openssl rand -base64 32`. |
| `OLD_API_KEYS` | empty | Old API keys being rotated out (comma-separated). Both current and old keys are accepted during rotation. |
| `REQUEST_SIZE_LIMIT` | `10MB` | Maximum request body size (human-readable format like "10MB", "100KB", "1GB") |
| `API_RATE_LIMIT_GLOBAL` | `100` | Global requests per second across all IPs (0 = no global limit) |
| `API_RATE_LIMIT_AUTH` | `50` | Requests per second per IP for authenticated endpoints (requires X-API-Key header) |
| `API_RATE_LIMIT_UNAUTH` | `10` | Requests per second per IP for unauthenticated endpoints (including `/health`) |
| `API_RATE_LIMIT_BURST` | `2` | Burst size multiplier (burst = rate × multiplier) |
| `BOT_STATUS` | `online` | Discord bot status (online, idle, dnd, invisible) |
| `BOT_ACTIVITY_TYPE` | `watching` | Activity type (playing, streaming, listening, watching, competing) |
| `BOT_ACTIVITY_TEXT` | `over your channels` | Activity text |
| `SECURITY_HSTS_MAX_AGE` | `31536000` | HSTS max-age in seconds (default: 1 year) |
| `SECURITY_CSP` | *(empty)* | Content-Security-Policy header (optional) |
| `SECURITY_PERMISSIONS_POLICY` | *(empty)* | Permissions-Policy header (optional) |
| `CORS_ALLOWED_ORIGINS` | `*` | Allowed origins (comma-separated, default: "*" for development) |
| `CORS_ALLOWED_METHODS` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` | Allowed HTTP methods (comma-separated) |
| `CORS_ALLOWED_HEADERS` | `Content-Type, X-API-Key, Authorization` | Allowed headers (comma-separated) |
| `CORS_EXPOSED_HEADERS` | *(empty)* | Exposed headers (comma-separated, optional) |
| `CORS_MAX_AGE` | `86400` | Max age for preflight cache in seconds (default: 24 hours) |
| `CORS_ALLOW_CREDENTIALS` | `true` | Allow credentials (cookies/authentication) (default: true for development) |

### API Key Security

The API supports multiple API keys and key rotation for enhanced security:

- **Multiple keys**: Provide comma-separated values in `API_KEY` environment variable. All keys are accepted.
- **Key rotation**: During rotation, add the new key to `API_KEY` and move the old key to `OLD_API_KEYS`. Both sets of keys are accepted, allowing seamless rotation without downtime.
- **Key generation**: Generate secure random keys using:
  ```bash
  openssl rand -base64 32 | tr -d '\n'
  ```
- **Key validation**: Keys must be at least 32 characters long, contain at least two character classes (upper/lower/digit/special), and must not contain weak substrings (e.g., default values, common strings). Weak keys are rejected at startup.
- **Audit logging**: Authentication events log masked keys (first 4 characters) for identification.

### Production Security Checklist

When deploying to production, ensure the following security measures are in place:

1. **Environment Configuration**
   - Set `GO_ENV=production` to enable production-specific security warnings
   - Remove `DISCORD_DEV_GUILD_ID` to register commands globally
   - Use strong, randomly generated API keys (minimum 32 characters, multiple character classes)
   - Set `DB_SSLMODE=require` or `verify-full` for encrypted database connections
   - Configure `CORS_ALLOWED_ORIGINS` to specific domains (avoid `*`)
   - Set `CORS_ALLOW_CREDENTIALS=false` unless cross-origin credentials are required
   - Consider setting a `SECURITY_CSP` header for additional XSS protection

2. **Runtime Validation**
   - The application performs automatic security validation on startup and logs warnings for potential issues
   - Warnings are logged for insecure configurations (CORS wildcards, disabled SSL, weak keys, etc.)
   - Validation does **not** block startup but provides actionable guidance for operators

3. **Monitoring and Logging**
   - Review application logs for security warnings after deployment
   - Monitor audit logs for suspicious activity (`module="audit"`)
   - Use `AUDIT_LOG_LEVEL=info` or higher in production (avoid `debug`)

4. **Infrastructure Security**
   - Run the bot and API behind a reverse proxy (nginx, Caddy) for TLS termination
   - Use firewall rules to restrict database access to trusted sources
   - Regularly rotate API keys and Discord tokens
   - Keep dependencies updated via `go mod tidy` and security scanning

### Security Headers

The API automatically adds security headers to all HTTP responses:

- **Strict-Transport-Security (HSTS)** – Enforces HTTPS with configurable `max‑age` (default: 1 year)
- **X‑Frame‑Options: DENY** – Prevents click‑jacking
- **X‑Content‑Type‑Options: nosniff** – Prevents MIME sniffing
- **X‑XSS‑Protection: 0** – Disables legacy XSS filter (replaced by CSP)
- **Referrer‑Policy: strict‑origin‑when‑cross‑origin** – Limits referrer information
- **Permissions‑Policy: camera=(), microphone=(), geolocation=()** – Restricts sensitive APIs
- **Content‑Security‑Policy: default‑src 'self'; style‑src 'self' 'unsafe‑inline'** – Controls resource loading; allows inline styles (required for some UI frameworks)

Customize via environment variables `SECURITY_CSP` and `SECURITY_PERMISSIONS_POLICY`.

Additionally, the `/health` endpoint includes `Cache-Control: no-store, max-age=0` and removes the `Server` header to limit information disclosure.

### Input Validation and Sanitization

The API includes comprehensive input validation and sanitization to prevent injection attacks (XSS, SQL injection, path traversal, log injection). All user input is validated and sanitized before processing.

**Validation (`internal/api/validation/`):**
- Discord ID validation (17‑19 digit numeric strings)
- UUID validation (with/without hyphens)
- Required field validation
- Length constraints
- Module name validation (alphanumeric + underscores)
- Emoji validation (Unicode and custom Discord formats)

**Sanitization (`internal/api/validation/sanitize.go`):**
- HTML escaping (`SanitizeHTML`) – prevents XSS in web output
- Path sanitization (`SanitizePath`) – prevents directory traversal
- Log sanitization (`SanitizeForLog`, `SanitizeLogDetails`) – removes control characters and limits length to prevent log injection
- String normalization (`NormalizeString`) – trims whitespace, normalizes Unicode, collapses multiple spaces

**Security principles:**
- Validate input early, sanitize before use
- Escape output according to context (HTML, JSON, logs)
- Use parameterized queries (via sqlc) to prevent SQL injection
- Audit logs sanitize user‑provided strings before writing

### CORS (Cross‑Origin Resource Sharing)

The API includes configurable CORS middleware that allows cross‑origin requests from web applications.

**Configuration via environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `CORS_ALLOWED_ORIGINS` | `*` | Allowed origins (comma‑separated). Use `*` for development, restrict to specific origins in production. |
| `CORS_ALLOWED_METHODS` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` | Allowed HTTP methods. |
| `CORS_ALLOWED_HEADERS` | `Content‑Type, X‑API‑Key, Authorization` | Allowed request headers. |
| `CORS_EXPOSED_HEADERS` | *(empty)* | Response headers exposed to the client (optional). |
| `CORS_MAX_AGE` | `86400` | Preflight cache duration in seconds (default: 24 h). |
| `CORS_ALLOW_CREDENTIALS` | `true` | Allow cookies/authentication headers (default: true for development). |

**Behaviour:**
- Preflight `OPTIONS` requests are handled automatically.
- When `CORS_ALLOW_CREDENTIALS` is `true` and origins are restricted, the `Access‑Control‑Allow‑Origin` header echoes the request origin (wildcard `*` is not allowed with credentials).
- The middleware logs rejected origins at debug level.

### Request Size Limiting

The API includes request size limiting middleware to protect against large payloads (DoS). The middleware rejects requests with body size exceeding the configured limit, returning HTTP 413 Payload Too Large.

**Configuration:**
- `REQUEST_SIZE_LIMIT` – Maximum request body size (human-readable format like "10MB", "100KB", "1GB"). Default: "10MB".

**Behaviour:**
- Requests with `Content-Length` header exceeding the limit are rejected immediately.
- Request bodies are buffered up to the limit+1 bytes to detect overflow.
- Oversized requests are logged with request ID for monitoring.

### API Rate Limiting

The API includes IP‑based rate limiting with a token bucket algorithm to protect against excessive requests. Different limits are applied for authenticated vs unauthenticated endpoints, and a global limit can be enforced across all clients.

**Configuration:**

| Variable | Default | Description |
|----------|---------|-------------|
| `API_RATE_LIMIT_GLOBAL` | `100` | Global requests per second across all IPs (0 = no global limit) |
| `API_RATE_LIMIT_AUTH` | `50` | Requests per second per IP for authenticated endpoints (requires X-API-Key header) |
| `API_RATE_LIMIT_UNAUTH` | `10` | Requests per second per IP for unauthenticated endpoints (including `/health`) |
| `API_RATE_LIMIT_BURST` | `2` | Burst size multiplier (burst = rate × multiplier) |

**Behaviour:**
- Client IP is extracted from the `X‑Forwarded‑For` header (when behind a proxy) or `RemoteAddr`.
- The health endpoint (`/health`) is always considered unauthenticated.
- When a rate limit is exceeded, the middleware responds with HTTP 429 Too Many Requests and includes a `Retry‑After` header.
- Rate‑limited requests are logged with client IP and endpoint.
- Per‑IP limiters are automatically cleaned up after 10 minutes of inactivity.

**Middleware order:** CORS → Security → SizeLimit → RateLimit → Recovery → Logging → Auth → Audit → Routes

### Audit Logging

The API includes an audit logging system that records security-critical events for forensic analysis. Audit logs are immutable and provide a detailed trail of authentication attempts, administrative actions, and sensitive data access.

**Configuration:**
- `AUDIT_LOG_LEVEL` – Minimum log level for audit events (`debug`, `info`, `warn`, `error`). Default: `info`.

**Events tracked:**
- Authentication success/failure (missing/invalid API key)
- Module enable/disable operations
- Sensitive data reads (guilds, modules, JTC config)
- Sensitive data writes (JTC parent channel create/delete)

**Implementation:**
- Audit logs are emitted as structured JSON (or text) via the standard logging infrastructure.
- Each audit event includes request ID, API key (masked), IP address, user agent, timestamp, and event-specific details.
- Audit logs are written to the same output as application logs but can be separated by filtering on the `module="audit"` field.

**Middleware order:** CORS → Security → SizeLimit → RateLimit → Recovery → Logging → Auth → Audit → Routes

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
    OnEnable(ctx context.Context, guildID string) error
    OnDisable(ctx context.Context, guildID string) error
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

### ✅ Reaction Roles

Assign roles to users who react to specific messages with designated emojis.

**Setup:** `/reactionrole add message_id:<id> emoji:<emoji> role:<role>`

**Features:**
- Supports both Unicode emojis and custom emojis
- Bot automatically reacts to messages when creating reaction roles
- Role assignment when users react, role removal when users remove reactions
- Cleanup functionality to remove invalid reactions and sync with current role assignments
- List all reaction roles sorted by channel then message ID

**Commands:**
- `/reactionrole add message_id:<id> emoji:<emoji> role:<role>` — Add reaction role to specific message
- `/reactionrole remove message_id:<id> emoji:<emoji>` — Remove reaction role from message
- `/reactionrole list` — List all reaction roles in the server (sorted by channel then message)
- `/reactionrole fix message_id:<id>` — Cleanup reactions on a message:
  - Removes reactions from users who don't have the associated role
  - Removes reactions with unauthorized emojis
  - Keeps valid reactions from users with the correct role
  - Ensures bot's reactions are present for new users

**Emoji Formats:**
- Unicode emojis: `✅`, `🔥`, `🎉`
- Custom emojis: `:emoji_name:` or `:emoji_name:1234567890`

### 🔲 Auto Role *(scaffolded)*

Automatically assign roles on join or first activity. Coming soon.

## API Reference

All API endpoints (except `/health`) require the `X-API-Key` header.

The health endpoint is secured to limit information disclosure: server header removed, caching disabled, only GET method allowed, and minimal response content.

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
