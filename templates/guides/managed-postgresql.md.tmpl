---
page_title: "Using the Provider with Managed PostgreSQL Services"
subcategory: "Guides"
description: |-
  How to connect the Terraform PostgreSQL provider to AWS RDS, GCP Cloud SQL, and Azure Database for PostgreSQL, including SSL and IAM authentication.
---

# Using the Provider with Managed PostgreSQL Services

This guide covers how to configure the PostgreSQL provider for managed database services from AWS, Google Cloud, and Azure. Managed services have specific requirements around SSL, superuser access, and authentication that differ from a self-hosted PostgreSQL instance.

## Key Differences from Self-Hosted PostgreSQL

Managed PostgreSQL services typically:

- **Do not provide true superuser access.** The admin user you receive has elevated privileges but is not a PostgreSQL superuser. Set `superuser = false` in the provider to avoid operations that require superuser.
- **Require SSL connections.** Most services enforce or strongly recommend encrypted connections.
- **May use IAM-based authentication** instead of or in addition to password authentication.

## AWS RDS for PostgreSQL

### Basic Configuration

```terraform
provider "postgresql" {
  host     = "mydb.abc123xyz.us-east-1.rds.amazonaws.com"
  port     = 5432
  username = "postgres"
  password = var.rds_password
  database = "postgres"
  sslmode  = "require"

  superuser = false
}
```

### With SSL Certificate Verification

For `verify-full` SSL mode, download the [RDS CA certificate bundle](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html):

```terraform
provider "postgresql" {
  host     = "mydb.abc123xyz.us-east-1.rds.amazonaws.com"
  port     = 5432
  username = "postgres"
  password = var.rds_password
  database = "postgres"
  sslmode  = "verify-full"

  sslrootcert = "${path.module}/certs/rds-ca-bundle.pem"

  superuser = false
}
```

### IAM Database Authentication

AWS RDS supports [IAM database authentication](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html), where a short-lived token replaces the password. You can generate the token outside of Terraform and pass it in:

```terraform
variable "rds_auth_token" {
  type      = string
  sensitive = true
  description = "IAM auth token generated via 'aws rds generate-db-auth-token'"
}

provider "postgresql" {
  host     = "mydb.abc123xyz.us-east-1.rds.amazonaws.com"
  port     = 5432
  username = "iam_user"
  password = var.rds_auth_token
  database = "postgres"
  sslmode  = "require"

  superuser = false
}
```

Generate the token before running Terraform:

```shell
export TF_VAR_rds_auth_token=$(aws rds generate-db-auth-token \
  --hostname mydb.abc123xyz.us-east-1.rds.amazonaws.com \
  --port 5432 \
  --username iam_user \
  --region us-east-1)

terraform apply
```

> **Note:** IAM auth tokens expire after 15 minutes. Generate a fresh token each time you run Terraform.

### Connection Pooling for RDS

RDS instances have connection limits based on instance size. Configure the provider connection pool to avoid exhausting them:

```terraform
provider "postgresql" {
  host     = "mydb.abc123xyz.us-east-1.rds.amazonaws.com"
  port     = 5432
  username = "postgres"
  password = var.rds_password
  sslmode  = "require"

  superuser            = false
  max_connections      = 4
  max_idle_connections  = 2
  conn_max_lifetime    = "10m"
  conn_max_idle_time   = "5m"
}
```

## GCP Cloud SQL for PostgreSQL

### Basic Configuration

```terraform
provider "postgresql" {
  host     = "10.0.0.5" # Private IP or Cloud SQL Proxy address
  port     = 5432
  username = "postgres"
  password = var.cloudsql_password
  database = "postgres"
  sslmode  = "require"

  superuser = false
}
```

### Using the Cloud SQL Auth Proxy

The recommended way to connect to Cloud SQL is through the [Cloud SQL Auth Proxy](https://cloud.google.com/sql/docs/postgres/connect-auth-proxy). The proxy handles SSL and IAM authentication, so the provider connects to `localhost`:

```terraform
provider "postgresql" {
  host     = "127.0.0.1"
  port     = 5432
  username = "postgres"
  password = var.cloudsql_password
  database = "postgres"
  sslmode  = "disable" # SSL is handled by the proxy

  superuser = false
}
```

Start the proxy before running Terraform:

```shell
cloud-sql-proxy myproject:us-central1:myinstance &

terraform apply
```

### Direct SSL Connection

If connecting directly without the proxy, use the server CA and client certificates from the Cloud SQL console:

```terraform
provider "postgresql" {
  host     = "34.123.45.67"
  port     = 5432
  username = "postgres"
  password = var.cloudsql_password
  database = "postgres"
  sslmode  = "verify-ca"

  sslcert     = "${path.module}/certs/client-cert.pem"
  sslkey      = "${path.module}/certs/client-key.pem"
  sslrootcert = "${path.module}/certs/server-ca.pem"

  superuser = false
}
```

### Cloud SQL IAM Authentication

Cloud SQL supports [IAM database authentication](https://cloud.google.com/sql/docs/postgres/authentication). For IAM users, generate an access token:

```shell
export TF_VAR_cloudsql_iam_token=$(gcloud auth print-access-token)

terraform apply
```

```terraform
variable "cloudsql_iam_token" {
  type      = string
  sensitive = true
}

provider "postgresql" {
  host     = "127.0.0.1" # via Cloud SQL Auth Proxy
  port     = 5432
  username = "iam-user@myproject.iam"
  password = var.cloudsql_iam_token
  database = "postgres"
  sslmode  = "disable"

  superuser = false
}
```

## Azure Database for PostgreSQL

### Basic Configuration (Flexible Server)

```terraform
provider "postgresql" {
  host     = "myserver.postgres.database.azure.com"
  port     = 5432
  username = "adminuser"
  password = var.azure_password
  database = "postgres"
  sslmode  = "require"

  superuser = false
}
```

> **Note:** Azure Database for PostgreSQL Flexible Server uses the username as-is. The older Single Server required the format `user@servername`, but this is not needed for Flexible Server.

### With SSL Certificate Verification

Download the [Azure CA certificate](https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-networking-ssl-tls) for full verification:

```terraform
provider "postgresql" {
  host     = "myserver.postgres.database.azure.com"
  port     = 5432
  username = "adminuser"
  password = var.azure_password
  database = "postgres"
  sslmode  = "verify-full"

  sslrootcert = "${path.module}/certs/DigiCertGlobalRootCA.crt.pem"

  superuser = false
}
```

### Microsoft Entra (Azure AD) Authentication

Azure supports [Microsoft Entra authentication](https://learn.microsoft.com/en-us/azure/postgresql/flexible-server/concepts-azure-ad-authentication) for PostgreSQL. Generate an access token:

```shell
export TF_VAR_azure_ad_token=$(az account get-access-token \
  --resource-type oss-rdbms \
  --query accessToken -o tsv)

terraform apply
```

```terraform
variable "azure_ad_token" {
  type      = string
  sensitive = true
}

provider "postgresql" {
  host     = "myserver.postgres.database.azure.com"
  port     = 5432
  username = "entra-admin@mydomain.com"
  password = var.azure_ad_token
  database = "postgres"
  sslmode  = "require"

  superuser = false
}
```

## The `superuser` Attribute

When set to `false`, the provider avoids executing SQL statements that require superuser privileges. This is essential for managed services where your admin user does not have true superuser access.

Behavior with `superuser = false`:

- Role creation does not attempt to set superuser-only attributes.
- Some provider operations may be limited compared to a true superuser connection.

```terraform
provider "postgresql" {
  # ... connection settings ...
  superuser = false
}
```

If you do not set `superuser`, the provider defaults to assuming superuser access is available. Always set `superuser = false` for managed services.

## The `expected_version` Attribute

In some environments (such as when connecting through a connection pooler like PgBouncer), the provider cannot reliably detect the PostgreSQL server version. Use `expected_version` to tell the provider which version to target:

```terraform
provider "postgresql" {
  host     = "pgbouncer.internal"
  port     = 6432
  username = "postgres"
  password = var.db_password
  sslmode  = "require"

  superuser        = false
  expected_version = "15.0"
}
```

## Summary of Provider Attributes for Managed Services

| Attribute | AWS RDS | GCP Cloud SQL | Azure Flexible Server |
|-----------|---------|---------------|-----------------------|
| `sslmode` | `require` or `verify-full` | `require` / `verify-ca` (direct), `disable` (proxy) | `require` or `verify-full` |
| `sslrootcert` | RDS CA bundle | Server CA from console | DigiCert Global Root CA |
| `sslcert` / `sslkey` | Rarely used | For direct connections | Rarely used |
| `superuser` | `false` | `false` | `false` |
| `expected_version` | If using RDS Proxy | If using Cloud SQL Auth Proxy | If using PgBouncer |

## Next Steps

- [Getting Started Guide](getting-started) -- Basic provider setup for local development.
- [Access Control Guide](access-control) -- Set up roles, grants, and default privileges.
- [Importing Resources Guide](importing-resources) -- Import existing infrastructure into Terraform state.
