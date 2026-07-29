variable "zone" {
  description = "Zone to manage, with the trailing dot."
  type        = string
}

variable "ttl" {
  description = "TTL for the record."
  type        = number
  default     = 3600
}
