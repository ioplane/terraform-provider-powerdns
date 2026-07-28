# Every Recursor write needs webservice.api_dir. Without it the Recursor is
# read-only and this provider reports the setting rather than the status code.

resource "powerdns_recursor_zone" "internal" {
  name    = "corp.example."
  kind    = "Forwarded"
  servers = ["192.0.2.53", "192.0.2.54"]

  # Set when forwarding to a resolver, cleared when forwarding to an
  # authoritative server. Getting it wrong produces answers that look like a
  # broken upstream.
  recursion_desired = true
}

# The Recursor appends :53 to an address given without a port, so
# "192.0.2.53" and "192.0.2.53:53" are the same upstream and do not produce a
# diff. A different port is a real change.
