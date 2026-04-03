# Import with schema: owner/role/database/schema/object_type
terraform import postgresql_default_privileges.tables "app_owner/readonly/my_application/public/table"

# Import without schema (database-wide): owner/role/database/object_type
terraform import postgresql_default_privileges.global "app_owner/readonly/my_application/table"
