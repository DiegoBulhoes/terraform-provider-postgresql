provider "postgresql" {
  host     = "localhost"
  port     = 5432
  username = "postgres"
  password = "secret"
  database = "postgres"
  sslmode  = "prefer"
}
