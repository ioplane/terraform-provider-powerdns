# Adopting a record by identity rather than by a parsed string.
#
# `terraform import powerdns_record.www 'zone./name./A'` already has a scenario.
# This is the other half of the contract: the resource declares an identity
# schema, and a consumer may write that identity into an `import` block instead
# of learning the delimiter. Nothing in the project had ever done so, which
# means the identity schemas were declared, asserted against the contract table,
# and never used the way a user would use them.
#
# The zone is not managed here. It exists on the server before this runs, so the
# subject is one record and the identity that finds it — not a zone whose
# creation would make the record exist anyway.

terraform {
  # Resource identity in an import block. The engine is pinned to Terraform in
  # the unit for the same reason the actions unit is: OpenTofu has neither.
  required_version = ">= 1.12"
}

resource "powerdns_record" "adopted" {
  zone   = var.zone
  name   = "adopted.${var.zone}"
  type   = "A"
  ttl    = var.ttl
  values = var.addresses
}

# The whole point of the module. Three attributes, no delimiter, no parsing —
# and if the provider's identity schema disagrees with this by so much as an
# attribute name, `terraform plan` refuses the block outright.
import {
  to = powerdns_record.adopted

  identity = {
    zone_name   = var.zone
    record_name = "adopted.${var.zone}"
    record_type = "A"
  }
}
