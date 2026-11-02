# Database-level grant: role/object_type/database (3 parts, no schema)
terraform import postgresql_grant.db_connect "app_user/database/my_application"

# Schema-level grant: role/object_type/database/schema (4 parts)
terraform import postgresql_grant.schema_usage "app_user/schema/my_application/app_schema"

# Grant on all objects of a type: same 4-part format
terraform import postgresql_grant.tables "app_user/table/my_application/public"

# Grant on named objects: object names, comma-separated, as a fifth part
terraform import postgresql_grant.orders "app_user/table/my_application/public/orders,order_items"
