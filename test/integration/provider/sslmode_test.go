//go:build integration

package provider_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/internal/provider"
	"github.com/DiegoBulhoes/terraform-provider-postgresql/test/acctest"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestAccProvider_sslmodeWithoutServerSSL(t *testing.T) {
	acctest.PreCheck(t)
	port, err := strconv.Atoi(os.Getenv("PGPORT"))
	if err != nil {
		t.Fatalf("PGPORT: %s", err)
	}
	for _, mode := range []string{"prefer", "allow", "disable"} {
		t.Run(mode, func(t *testing.T) {
			wrapper := configureProvider(t, os.Getenv("PGHOST"), port, mode)
			if usesSSL(t, wrapper) {
				t.Errorf("sslmode=%s: expected a plain connection to a server without SSL", mode)
			}
		})
	}
}

func TestAccProvider_sslmodePreferUsesServerSSL(t *testing.T) {
	acctest.PreCheck(t)
	host, port := startSSLPostgres(t)
	for _, mode := range []string{"prefer", "require"} {
		t.Run(mode, func(t *testing.T) {
			wrapper := configureProvider(t, host, port, mode)
			if !usesSSL(t, wrapper) {
				t.Errorf("sslmode=%s: expected an SSL connection to a server with SSL", mode)
			}
		})
	}
}

func configureProvider(t *testing.T, host string, port int, sslmode string) *common.DBWrapper {
	t.Helper()
	ctx := context.Background()
	p := &provider.PostgreSQLProvider{}
	schemaResp := &fwprovider.SchemaResponse{}
	p.Schema(ctx, fwprovider.SchemaRequest{}, schemaResp)

	resp := &fwprovider.ConfigureResponse{}
	p.Configure(ctx, fwprovider.ConfigureRequest{
		Config: tfsdk.Config{Raw: providerConfig(host, port, sslmode), Schema: schemaResp.Schema},
	}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("sslmode=%s: Configure failed: %v", sslmode, resp.Diagnostics.Errors())
	}
	wrapper, ok := resp.ResourceData.(*common.DBWrapper)
	if !ok {
		t.Fatalf("unexpected provider data %T", resp.ResourceData)
	}
	return wrapper
}

func usesSSL(t *testing.T, w *common.DBWrapper) bool {
	t.Helper()
	var ssl bool
	err := w.QueryRowContext(context.Background(),
		`SELECT ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid()`).Scan(&ssl)
	if err != nil {
		t.Fatalf("reading pg_stat_ssl: %s", err)
	}
	return ssl
}

func providerConfig(host string, port int, sslmode string) tftypes.Value {
	str := func(v any) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	num := func(v any) tftypes.Value { return tftypes.NewValue(tftypes.Number, v) }
	return tftypes.NewValue(tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"host": tftypes.String, "port": tftypes.Number, "username": tftypes.String,
			"password": tftypes.String, "database": tftypes.String, "sslmode": tftypes.String,
			"sslcert": tftypes.String, "sslkey": tftypes.String, "sslrootcert": tftypes.String,
			"connect_timeout": tftypes.Number, "max_connections": tftypes.Number,
			"max_idle_connections": tftypes.Number, "conn_max_lifetime": tftypes.Number,
			"conn_max_idle_time": tftypes.Number, "expected_version": tftypes.String,
		},
	}, map[string]tftypes.Value{
		"host": str(host), "port": num(port), "username": str("postgres"),
		"password": str("postgres"), "database": str("postgres"), "sslmode": str(sslmode),
		"sslcert": str(nil), "sslkey": str(nil), "sslrootcert": str(nil),
		"connect_timeout": num(10), "max_connections": num(nil),
		"max_idle_connections": num(nil), "conn_max_lifetime": num(nil),
		"conn_max_idle_time": num(nil), "expected_version": str(nil),
	})
}

// startSSLPostgres starts a PostgreSQL container with ssl=on and a self-signed certificate.
func startSSLPostgres(t *testing.T) (string, int) {
	t.Helper()
	ctx := context.Background()
	certPEM, keyPEM := selfSignedCert(t)

	// The init script copies the key into PGDATA so that it is owned by postgres with mode 0600.
	initScript := `#!/bin/sh
set -e
cp /tmp/tls/server.crt /tmp/tls/server.key "$PGDATA"/
chmod 600 "$PGDATA"/server.key
echo "ssl = on" >> "$PGDATA"/postgresql.conf
`
	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        acctest.GetEnvOrDefault("POSTGRES_IMAGE", "postgres:16-alpine"),
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "postgres",
				"POSTGRES_DB":       "postgres",
			},
			Files: []testcontainers.ContainerFile{
				{Reader: strings.NewReader(certPEM), ContainerFilePath: "/tmp/tls/server.crt", FileMode: 0o644},
				{Reader: strings.NewReader(keyPEM), ContainerFilePath: "/tmp/tls/server.key", FileMode: 0o644},
				{Reader: strings.NewReader(initScript), ContainerFilePath: "/docker-entrypoint-initdb.d/ssl.sh", FileMode: 0o755},
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("starting SSL postgres: %s", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(ctx) })

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mapped, err := ctr.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	return host, mapped.Int()
}

func selfSignedCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return string(certPEM), string(keyPEM)
}
