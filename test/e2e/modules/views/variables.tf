variable "zone" {
  description = "The zone placed into the view."
  type        = string
}

variable "view" {
  description = "The view's name."
  type        = string
}

variable "network" {
  description = "The client prefix mapped to the view. Compared as a prefix, not as text."
  type        = string
}

variable "nameservers" {
  description = "Create-only."
  type        = list(string)
}
