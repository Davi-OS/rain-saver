package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testDBLockID serializes access to the shared test database across every
// package's test binary. Distinct from go-auth-service's own lock id
// (918_273_645) in case both test suites ever run against a shared
// Postgres instance — the advisory lock is server-wide, not per-database.
const testDBLockID = 573_918_264

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// These tests TRUNCATE every table, so they never fall back to a default
	// DSN (that used to be the dev database) and only run against a database
	// explicitly named *_test.
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set: skipping Postgres tests (point it at a throwaway *_test database)")
	}
	pgCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}
	if db := pgCfg.ConnConfig.Database; !strings.HasSuffix(db, "_test") {
		t.Fatalf("TEST_DATABASE_URL points at database %q: refusing to run, these tests TRUNCATE every table; use a database whose name ends in _test", db)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("postgres not reachable: %v", err)
	}
	t.Cleanup(pool.Close)

	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire lock connection: %v", err)
	}
	if _, err := lockConn.Exec(ctx, `SELECT pg_advisory_lock($1)`, testDBLockID); err != nil {
		lockConn.Release()
		t.Fatalf("acquire advisory lock: %v", err)
	}
	t.Cleanup(func() {
		_, _ = lockConn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, testDBLockID)
		lockConn.Release()
	})

	_, err = pool.Exec(ctx, `TRUNCATE TABLE
		pluvial_monthly_captation, pluvial_economy, enderecos,
		station_monthly_rainfall, weather_stations RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate failed: %v", err)
	}

	return pool
}
