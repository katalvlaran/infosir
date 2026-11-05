package db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"infosir/cmd/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InitDatabase preserves the original entry point with bounded startup.
func InitDatabase() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return InitDatabaseContext(ctx)
}

// InitDatabaseContext opens the pool and applies the configured Timescale migrations.
func InitDatabaseContext(ctx context.Context) (*pgxpool.Pool, error) {
	c := config.Cfg.Database
	u := url.URL{Scheme: "postgres", User: url.UserPassword(c.User, c.Password), Host: net.JoinHostPort(c.Host, strconv.Itoa(c.Port)), Path: "/" + c.Name}
	q := u.Query()
	q.Set("sslmode", c.SSLMode)
	q.Set("connect_timeout", "5")
	q.Set("statement_timeout", "20000")
	u.RawQuery = q.Encode()
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	success := false
	defer func() {
		if !success {
			pool.Close()
		}
	}()
	if err = pool.Ping(ctx); err != nil {
		return nil, errors.New("database connection failed")
	}
	if err = runMigrations(u.String()); err != nil {
		return nil, fmt.Errorf("database migration: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	success = true
	return pool, nil
}
func runMigrations(dsn string) error {
	path, err := filepath.Abs(config.Cfg.Database.MigrationsPath)
	if err != nil {
		return err
	}
	source := url.URL{Scheme: "file", Path: path}
	m, err := migrate.New(source.String(), dsn)
	if err != nil {
		return errors.New("cannot open migration source or database")
	}
	m.LockTimeout = 10 * time.Second
	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		err = nil
	}
	sourceErr, dbErr := m.Close()
	return errors.Join(err, sourceErr, dbErr)
}
