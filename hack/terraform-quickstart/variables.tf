variable "access_key_id" {
  type = "string"
}

variable "access_key" {
  type = "string"
}

variable "ssh_public_key" {
  description = "SSH Public Key"
  type        = "string"
}

variable "resource_owner" {
  description = "Tag all resources behind a single tag based on who/what is running terraform"
  type        = "string"
  default     = "bootkube-terraform-example-deleteme"
}

variable "instance_type" {
  description = "Instance type"
  type        = "string"
  default     = "m3.medium"
}

variable "network_provider" {
  type    = "string"
  default = "flannel"
}

variable "num_workers" {
  description = "number of worker nodes"
  type        = "string"
  default     = "1"
}

variable "additional_masters" {
  description = "number of additional master nodes not including bootstrap node"
  type        = "string"
  default     = "0"
}

variable "region" {
  description = "aws region"
  type        = "string"
  default     = "us-east-1"
}

variable "environment" {
  description = "environment flavor to push to instances"
  type        = "string"
  default     = "default"
}
