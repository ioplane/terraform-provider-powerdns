output "zone_id" {
  description = "The zone the actions ran against."
  value       = powerdns_zone.primary.id
}

# There is no output for the ephemeral secret, and there cannot be. An ordinary
# output derived from an ephemeral value is refused ("not declared as returning
# an ephemeral value"); declaring it ephemeral is refused too, because a root
# module has nowhere to return one to. Both errors were met in that order
# writing this file.
#
# That is the property, enforced by the language rather than by the provider:
# a secret read this way has nowhere to go. The check block in main.tf is how
# the module establishes it was really read.
