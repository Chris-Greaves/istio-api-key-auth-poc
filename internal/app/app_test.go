package app_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/app"
	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/config"
)

func startPostgres(t *testing.T) string {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("apikeys"),
		postgres.WithUsername("apikeys"),
		postgres.WithPassword("apikeys"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminating postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting postgres connection string: %v", err)
	}

	return connStr
}

func TestApp_HealthyAndSchemaMigratedAfterStartup(t *testing.T) {
	connStr := startPostgres(t)
	ctx := context.Background()

	application, err := app.New(ctx, config.Config{DatabaseURL: connStr})
	if err != nil {
		t.Fatalf("starting application: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Logf("closing application: %v", err)
		}
	})

	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("calling /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected /healthz to return %d, got %d", http.StatusOK, resp.StatusCode)
	}

	verifyDB, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("opening verification connection: %v", err)
	}
	defer verifyDB.Close()

	var exists bool
	err = verifyDB.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'keys')`,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking for keys table: %v", err)
	}
	if !exists {
		t.Fatal("expected keys table to exist after migrations ran")
	}
}

func TestApp_HealthzReportsUnhealthyOnceDatabaseIsClosed(t *testing.T) {
	connStr := startPostgres(t)
	ctx := context.Background()

	application, err := app.New(ctx, config.Config{DatabaseURL: connStr})
	if err != nil {
		t.Fatalf("starting application: %v", err)
	}

	server := httptest.NewServer(application.Handler())
	t.Cleanup(server.Close)

	if err := application.Close(); err != nil {
		t.Fatalf("closing application: %v", err)
	}

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("calling /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected /healthz to return %d once the database connection is closed, got %d", http.StatusServiceUnavailable, resp.StatusCode)
	}
}

func TestNew_FailsFastWhenDatabaseIsUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := app.New(ctx, config.Config{
		DatabaseURL: "postgres://user:pass@127.0.0.1:1/nonexistent?sslmode=disable",
	})
	if err == nil {
		t.Fatal("expected an error when the database is unreachable, got nil")
	}
}
