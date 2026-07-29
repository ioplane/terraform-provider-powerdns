variable "forward_zone" {
  description = "The zone the Recursor forwards."
  type        = string
}

variable "upstreams" {
  description = "Upstream servers, compared semantically: a bare address means port 53."
  type        = list(string)
}

variable "recursor_netmasks" {
  description = "allow-from for the Recursor."
  type        = list(string)
}

variable "dnsdist_netmasks" {
  description = "allow-from for dnsdist."
  type        = list(string)
}
