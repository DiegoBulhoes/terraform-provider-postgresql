resource "postgresql_role" "readers" {
  name = "readers"
}

resource "postgresql_user" "analyst" {
  name = "analyst"
}

resource "postgresql_user" "team_lead" {
  name = "team_lead"
}

# Make analyst a member of readers
resource "postgresql_role_membership" "analyst_readers" {
  role   = postgresql_role.readers.name # Required: the role being granted
  member = postgresql_user.analyst.name # Required: the role receiving it
}

# A member that can grant the role to others
resource "postgresql_role_membership" "lead_readers" {
  role         = postgresql_role.readers.name
  member       = postgresql_user.team_lead.name
  admin_option = true # Optional, default false
}
