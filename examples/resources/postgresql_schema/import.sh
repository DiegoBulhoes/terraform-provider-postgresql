# Import by schema name
terraform import postgresql_schema.app app_schema

# Import with database prefix
terraform import postgresql_schema.app my_database/app_schema
