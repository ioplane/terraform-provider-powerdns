# For backups, and for diffing against a file kept in version control. The
# whole zone lands in state, so this is not something to run against a zone
# with thousands of records on every refresh.
data "powerdns_zone_export" "backup" {
  zone = "example.com."
}

resource "local_file" "zone_backup" {
  filename = "${path.module}/backups/example.com.zone"
  content  = data.powerdns_zone_export.backup.zone_file
}
