# The only configuration dnsdist's API can write. Rules, pools, downstream
# servers and dynamic blocks are Lua or YAML and have no HTTP path at all.
#
# Needs setAPIWritable(true, dir) in the Lua configuration; apiConfigDir alone
# is not enough, and without it every PUT answers 405.

resource "powerdns_dnsdist_acl" "clients" {
  netmasks = ["10.0.0.0/8", "127.0.0.0/8", "::1/128"]
}
