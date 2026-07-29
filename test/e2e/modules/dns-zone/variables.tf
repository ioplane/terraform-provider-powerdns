variable "zone" {
  description = "The forward zone, fully qualified with a trailing dot."
  type        = string
}

variable "host" {
  description = "The host label within the zone."
  type        = string
  default     = "www"
}

variable "cidr" {
  description = "The network the reverse zone is derived from."
  type        = string
}

variable "addresses" {
  description = "The addresses the host resolves to. More than one is the point: an RRSet holds a set."
  type        = list(string)
}

variable "ttl" {
  description = "One TTL applies to a whole RRSet; DNS has no per-record TTL."
  type        = number
  default     = 3600
}

variable "nameservers" {
  description = "Create-only. PowerDNS does not report them back, so a change here cannot be detected."
  type        = list(string)
}
