variable "zone" {
  description = "The zone the actions operate on."
  type        = string
}

variable "nameservers" {
  description = "Create-only."
  type        = list(string)
}

variable "autoprimary_ip" {
  description = "The autoprimary's address."
  type        = string
}

variable "autoprimary_ns" {
  description = "The autoprimary's nameserver name."
  type        = string
}

variable "tsig_name" {
  description = "A TSIG key whose secret is read back ephemerally."
  type        = string
}
