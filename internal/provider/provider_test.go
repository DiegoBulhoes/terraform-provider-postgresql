package provider

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"postgresql": providerserver.NewProtocol6WithError(New("test")()),
}

var (
	testAccDB   *sql.DB
	testAccOnce sync.Once
)

// testAccGetDB returns a shared *sql.DB connection for acceptance test helpers
// such as CheckDestroy functions.
func testAccGetDB() (*sql.DB, error) {
	var err error
	testAccOnce.Do(func() {
		host := getEnvOrDefault("PGHOST", "localhost")
		port := getEnvOrDefault("PGPORT", "5432")
		user := getEnvOrDefault("PGUSER", "postgres")
		password := getEnvOrDefault("PGPASSWORD", "postgres")
		dbname := getEnvOrDefault("PGDATABASE", "postgres")
		sslmode := getEnvOrDefault("PGSSLMODE", "disable")

		connStr := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode,
		)

		testAccDB, err = sql.Open("postgres", connStr)
		if err == nil {
			testAccDB.SetMaxOpenConns(3)
			testAccDB.SetMaxIdleConns(1)
		}
	})
	return testAccDB, err
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func TestMain(m *testing.M) {
	// If PGHOST is already set, skip container setup (use external database).
	if os.Getenv("PGHOST") != "" {
		os.Exit(m.Run())
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
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

	os.Setenv("PGHOST", host)
	os.Setenv("PGPORT", port.Port())
	os.Setenv("PGUSER", "postgres")
	os.Setenv("PGPASSWORD", "postgres")
	os.Setenv("PGDATABASE", "postgres")
	os.Setenv("PGSSLMODE", "disable")

	os.Exit(m.Run())
}
