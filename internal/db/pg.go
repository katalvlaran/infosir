package db

import (
	"context"
	"fmt"
	"strings"

	"infosir/cmd/config"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is the old function that just returns *pgxpool.Pool
func InitDB() (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.Cfg.DB.DBUser, config.Cfg.DB.DBPass,
		config.Cfg.DB.DBHost, config.Cfg.DB.DBPort,
		config.Cfg.DB.DBName,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping Postgres: %w", err)
	}
	return pool, nil
}

// This new function calls InitDB, then auto-creates tables
func InitDatabase() (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.Cfg.DB.DBUser, config.Cfg.DB.DBPass,
		config.Cfg.DB.DBHost, config.Cfg.DB.DBPort,
		config.Cfg.DB.DBName,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	if err = pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping Postgres: %w", err)
	}

	m, err := migrate.New("file://migrations", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrate: %w", err)
	}

	err = m.Up()
	if err != nil && err != migrate.ErrNoChange {
		return nil, fmt.Errorf("failed to run migrate: %w", err)
	}

	fmt.Println("Migration complete!")

	return pool, nil
}

func pairToTable(pair string) string {
	return strings.ToLower(pair)
}
