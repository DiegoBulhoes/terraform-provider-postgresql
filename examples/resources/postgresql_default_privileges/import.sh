# Defaults in one schema: owner/role/object_type/database/schema
terraform import postgresql_default_privileges.reader_tables "app_owner/app_reader/table/my_application/app"

# Defaults in every schema: owner/role/object_type/database
terraform import postgresql_default_privileges.functions "app_owner/app_admin/function/my_application"
