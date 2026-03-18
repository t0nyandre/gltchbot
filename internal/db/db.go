package db

import (
	"context"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/t0nyandre/gltchbot/internal/config"
	"github.com/t0nyandre/gltchbot/internal/logging"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// New creates a new pgxpool connection pool and runs all pending migrations.
func New(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	// Parse connection string into pgxpool config
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse connection config: %w", err)
	}

	// Apply connection pool settings
	poolConfig.MaxConns = int32(cfg.DBMaxConns)
	poolConfig.MinConns = int32(cfg.DBMinConns)
	poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.DBMaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := runMigrations(cfg); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return pool, nil
}

// runMigrations applies all pending up migrations using golang-migrate.
func runMigrations(cfg *config.Config) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	// golang-migrate pgx/v5 driver uses a URL-style connection string
	dbURL := fmt.Sprintf("pgx5://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}

	logging.Info("database migrations applied successfully", "component", "database")
	return nil
}
