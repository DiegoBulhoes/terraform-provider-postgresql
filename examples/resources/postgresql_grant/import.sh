# Import database grant: role/object_type/database/
terraform import postgresql_grant.db_connect "app_user/database/my_application/"

# Import schema grant: role/object_type/database/schema
terraform import postgresql_grant.schema_usage "app_user/schema//app_schema"
