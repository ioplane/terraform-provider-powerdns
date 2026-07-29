variable "zone" {
  description = "Zone the record lives in, with the trailing dot. Not managed here."
  type        = string
}

variable "addresses" {
  description = "Values the record is expected to already have."
  type        = list(string)
}

variable "ttl" {
  description = "TTL the record is expected to already have."
  type        = number
}
