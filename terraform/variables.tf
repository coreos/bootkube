variable "cluster_name" {
  description = "Cluster name"
  type        = "string"
}

variable "api_servers" {
  description = "URL used to reach kube-apiserver"
  type        = "list"
}

variable "etcd_servers" {
  description = "List of etcd server URLs including protocol, host, and port"
  type        = "list"
}

variable "asset_dir" {
  description = "Output path where generated assets should be placed (contains secrets)"
  type        = "string"
}

variable "cloud_provider" {
  description = "The provider for cloud services (empty string for no provider)"
  type        = "string"
  default     = ""
}

variable "pod_cidr" {
  description = "CIDR IP range to assign Kubernetes pods"
  type        = "string"
  default     = "10.2.0.0/16"
}

variable "service_cidr" {
  description = "CIDR IP range to assign Kubernetes services"
  type        = "string"
  default     = "10.3.0.0/24"
}

variable "container_images" {
  description = "Container images to use"
  type        = "map"

  // Only template images which are used more than once
  default = {
    hyperkube = "quay.io/coreos/hyperkube:v1.6.2_coreos.0"
  }
}

variable "kube_apiserver_service_ip" {
  description = "Kubernetes service IP for kube-apiserver (must be within service_cidr)"
  type        = "string"
  default     = "10.3.0.1"
}

variable "kube_dns_service_ip" {
  description = "Kubernetes service IP for kube-dns (must be within server_cidr)"
  type        = "string"
  default     = "10.3.0.10"
}

variable "ca_certificate" {
  description = "Existing PEM-encoded CA certificate (generated if blank)"
  type        = "string"
  default     = ""
}

variable "ca_key_alg" {
  description = "Algorithm used to generate ca_key (required if ca_cert is specified)"
  type        = "string"
  default     = "RSA"
}

variable "ca_private_key" {
  description = "Existing Certificate Authority private key (required if ca_certificate is set)"
  type        = "string"
  default     = ""
}
