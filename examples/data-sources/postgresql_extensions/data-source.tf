# List all installed extensions
data "postgresql_extensions" "all" {}

output "installed_extensions" {
  value = {
    for ext in data.postgresql_extensions.all.extensions :
    ext.name => ext.version
  }
}
