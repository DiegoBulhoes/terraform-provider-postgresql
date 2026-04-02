# Basic database
resource "postgresql_database" "mydb" {
  name = "my_application"
}

# Database with full configuration
resource "postgresql_database" "configured" {
  name              = "my_app_db"
  owner             = postgresql_role.owner.name
  template          = "template0"
  encoding          = "UTF8"
  lc_collate        = "en_US.UTF-8"
  lc_ctype          = "en_US.UTF-8"
  tablespace_name   = "pg_default"
  connection_limit  = 100
  allow_connections = true
  is_template       = false
}
