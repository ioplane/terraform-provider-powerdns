# A serial in the conventional YYYYMMDDnn form.
#
# The date is an argument rather than read from the clock, because a function
# returning today's date would change the plan every day and never converge.
# The revision is 0 to 99: the convention has two digits for it.
output "serial" {
  value = provider::powerdns::soa_serial("2026-07-29", 1) # 2026072901
}
