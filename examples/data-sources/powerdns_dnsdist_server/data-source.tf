# dnsdist's downstreams, pools and rules are configured in Lua or YAML and have
# no HTTP write path. This is how a configuration observes what Lua decided.
data "powerdns_dnsdist_server" "edge" {}

output "unhealthy_backends" {
  value = [
    for d in data.powerdns_dnsdist_server.edge.downstreams :
    d.name if d.state != "up"
  ]
}
