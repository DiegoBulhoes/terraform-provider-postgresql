terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.2"
    }
  }
}

# Minimal: local PostgreSQL with explicit credentials
provider "postgresql" {
  host     = "localhost"
  port     = 5432
  username = "postgres"
  password = "secret"
  database = "postgres"
  sslmode  = "prefer"
}

# Environment-driven: reads PGHOST, PGPORT, PGUSER, PGPASSWORD, PGDATABASE, PGSSLMODE
provider "postgresql" {
  alias = "from_env"
}

# Managed service (RDS / Cloud SQL / Azure Database): superuser = false skips
# operations that require superuser privileges on the server.
provider "postgresql" {
  alias    = "managed"
  host     = "myapp.abc123.us-east-1.rds.amazonaws.com"
  port     = 5432
  username = "masteruser"
  password = var.db_password
  database = "postgres"
  sslmode  = "require"

  superuser = false
}

# TLS with client certificates. Paths containing spaces, single quotes, or
# backslashes are automatically escaped in the connection string.
provider "postgresql" {
  alias       = "mtls"
  host        = "db.example.com"
  username    = "app"
  password    = var.db_password
  database    = "app"
  sslmode     = "verify-full"
  sslcert     = "/etc/tls/client.crt"
  sslkey      = "/etc/tls/client.key"
  sslrootcert = "/etc/tls/ca.crt"
}

# Tuned connection pool for high-parallelism Terraform runs. All time-based
# attributes are integer seconds (not duration strings).
provider "postgresql" {
  alias    = "pooled"
  host     = "db.example.com"
  username = "postgres"
  password = var.db_password

  connect_timeout      = 30
  max_connections      = 20
  max_idle_connections = 10
  conn_max_lifetime    = 1800 # 30 minutes
  conn_max_idle_time   = 300  # 5 minutes
}

variable "db_password" {
  type      = string
  sensitive = true
}
