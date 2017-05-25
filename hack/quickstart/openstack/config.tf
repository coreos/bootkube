variable "flavor_id" {
  type    = "string"
  default = "bbcb7eb5-5c8d-498f-9d7e-307c575d3566"
}

variable "image_id" {
  type    = "string"
  default = "90f57210-9354-4a2f-852e-d844237fbbad"
}

variable "cluster_dir" {
  type = "string"
  default = "cluster"
}

variable "controller_count" {
  type    = "string"
  default = "1"
}

variable "worker_count" {
  type    = "string"
  default = "3"
}
