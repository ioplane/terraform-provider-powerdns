# The two products the lifecycle module never touches.
#
# Recursor and dnsdist are most of what makes this one provider rather than
# three, and no end-to-end path had ever reached them. Both are thin because
# their APIs are: the Recursor writes two settings and its zones, dnsdist
# writes one setting and a cache flush.

terraform {
  required_version = ">= 1.10"
  required_providers {
    powerdns = {
      source  = "registry.terraform.io/ioplane/powerdns"
      version = "0.1.1"
    }
  }
}

# A forward zone on the Recursor. `kind` is what decides whether the Recursor
# forwards or answers; the servers are upstreams, not nameservers.
resource "powerdns_recursor_zone" "forward" {
  name              = var.forward_zone
  kind              = "Forwarded"
  servers           = var.upstreams
  recursion_desired = true
}

# allow-from, one of exactly two settings the Recursor will write. Destroying
# this resource does not clear the ACL — the setting has no empty state, and
# the provider says so rather than pretending.
resource "powerdns_recursor_acl" "allow_from" {
  setting  = "allow-from"
  netmasks = var.recursor_netmasks
}

# dnsdist's single writable ACL. No `setting` attribute, and that is the
# schema being honest: dnsdist has exactly one ACL, so a field to choose which
# would only ever hold one value.
resource "powerdns_dnsdist_acl" "allow_from" {
  netmasks = var.dnsdist_netmasks
}

data "powerdns_recursor_zone" "readback" {
  name       = powerdns_recursor_zone.forward.name
  depends_on = [powerdns_recursor_zone.forward]
}

data "powerdns_dnsdist_server" "self" {}
