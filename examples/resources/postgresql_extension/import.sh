# Import by extension name
terraform import postgresql_extension.pgcrypto pgcrypto

# Import with database prefix
terraform import postgresql_extension.citext my_database/citext
