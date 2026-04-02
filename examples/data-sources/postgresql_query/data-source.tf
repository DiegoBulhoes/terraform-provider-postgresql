data "postgresql_query" "connections" {
  database = "my_application"
  query    = "SELECT usename, client_addr, state FROM pg_stat_activity WHERE datname = current_database()"
}

output "active_connections" {
  value = data.postgresql_query.connections.rows
}
