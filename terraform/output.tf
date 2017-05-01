output "id" {
  value = "${sha1("${template_dir.bootstrap-manifests.id} ${local_file.kubeconfig.id}")}"
}

output "content_hash" {
  value = "${sha1("${template_dir.bootstrap-manifests.id} ${template_dir.manifests.id}")}"
}

output "kubeconfig" {
  value = "${data.template_file.kubeconfig.rendered}"
}

output "user-kubeconfig" {
  value = "${local_file.user-kubeconfig.filename}"
}

# Some platforms may need to reconstruct the kubeconfig directly in user-data.
# That can't be done with the way template_file interpolates multi-line
# contents so the raw components of the kubeconfig may be needed.

output "ca_cert" {
  value = "${base64encode(var.ca_certificate == "" ? join(" ", tls_self_signed_cert.kube-ca.*.cert_pem) : var.ca_certificate)}"
}

output "kubelet_cert" {
  value = "${base64encode(tls_locally_signed_cert.kubelet.cert_pem)}"
}

output "kubelet_key" {
  value = "${base64encode(tls_private_key.kubelet.private_key_pem)}"
}

output "server" {
  value = "${format("https://%s:443", element(var.api_servers, 0))}"
}
