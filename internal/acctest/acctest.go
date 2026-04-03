// Package acctest provides shared test infrastructure for acceptance tests
// across all packages (provider, resource, datasource).
package acctest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ProviderFactories is set before tests run. It is populated lazily by
// ProviderFactoriesInit which is set by the provider package's init_test.go.
var ProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

// ProviderFactoriesInit is a function that creates the provider factories.
// Set this in an init() in a _test.go file in a package that can import provider
// without creating cycles (e.g. the provider package itself).
var ProviderFactoriesInit func() map[string]func() (tfprotov6.ProviderServer, error)

var (
	db   *sql.DB
	once sync.Once
)

// GetDB returns a shared *sql.DB connection for acceptance test helpers
// such as CheckDestroy functions.
func GetDB() (*sql.DB, error) {
	var err error
	once.Do(func() {
		host := GetEnvOrDefault("PGHOST", "localhost")
		port := GetEnvOrDefault("PGPORT", "5432")
		user := GetEnvOrDefault("PGUSER", "postgres")
		password := GetEnvOrDefault("PGPASSWORD", "postgres")
		dbname := GetEnvOrDefault("PGDATABASE", "postgres")
		sslmode := GetEnvOrDefault("PGSSLMODE", "disable")

		connStr := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode,
		)

		db, err = sql.Open("postgres", connStr)
		if err == nil {
			db.SetMaxOpenConns(3)
			db.SetMaxIdleConns(1)
			db.SetConnMaxIdleTime(30 * time.Second)
		}
	})
	return db, err
}

// GetEnvOrDefault returns the value of the environment variable or the fallback.
func GetEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// PreCheck verifies the test database is reachable before running tests.
func PreCheck(t *testing.T) {
	t.Helper()
	conn, err := GetDB()
	if err != nil {
		t.Fatalf("Failed to get test database connection: %s", err)
	}
	if err := conn.Ping(); err != nil {
		t.Fatalf("Failed to ping test database: %s", err)
	}
}

// SetupTestContainer starts a PostgreSQL container and sets environment variables.
// Call this from TestMain in each test package that needs acceptance tests.
// If PGHOST is already set, the container is skipped.
func SetupTestContainer(m *testing.M) {
	if os.Getenv("PGHOST") != "" {
		os.Exit(m.Run())
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        GetEnvOrDefault("POSTGRES_IMAGE", "postgres:16-alpine"),
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
			"POSTGRES_DB":       "postgres",
		},
		Cmd: []string{"postgres", "-c", "max_connections=500"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(30 * time.Second),
	}

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %s\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "failed to terminate postgres container: %s\n", err)
		}
	}()

	host, err := pgContainer.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get container host: %s\n", err)
		os.Exit(1)
	}

	port, err := pgContainer.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get container port: %s\n", err)
		os.Exit(1)
	}

	setEnv := func(key, value string) {
		if err := os.Setenv(key, value); err != nil {
			fmt.Fprintf(os.Stderr, "failed to set %s: %s\n", key, err)
			os.Exit(1)
		}
	}
	setEnv("PGHOST", host)
	setEnv("PGPORT", port.Port())
	setEnv("PGUSER", "postgres")
	setEnv("PGPASSWORD", "postgres")
	setEnv("PGDATABASE", "postgres")
	setEnv("PGSSLMODE", "disable")

	os.Exit(m.Run())
}
