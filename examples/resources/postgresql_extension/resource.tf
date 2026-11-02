# Minimal: only "name" is required
resource "postgresql_extension" "pgcrypto" {
  name = "pgcrypto" # Required
}

# Pinned version in its own schema
resource "postgresql_extension" "hstore" {
  name    = "hstore"                   # Required
  schema  = postgresql_schema.ext.name # Optional (defaults to the first schema in the search path)
  version = "1.8"                      # Optional (defaults to the control file's version)
}

# Install the extensions it requires (earthdistance needs cube)
resource "postgresql_extension" "earthdistance" {
  name           = "earthdistance" # Required
  create_cascade = true            # Optional, default: false
}

# Extension in a specific database, dropped with the objects that use it
resource "postgresql_extension" "citext" {
  name         = "citext"                           # Required
  database     = postgresql_database.analytics.name # Optional (defaults to provider database)
  drop_cascade = true                               # Optional, default: false
}
