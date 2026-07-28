# Every zone on the server. The list endpoint omits records, so this stays
# cheap even on a server holding thousands.
data "powerdns_zones" "all" {}

output "slave_zones" {
  value = [for z in data.powerdns_zones.all.zones : z.name if z.kind == "Slave"]
}
