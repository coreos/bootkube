output "bootstrap_node_ip" {
  value = "${aws_instance.bootstrap_node.public_ip}"
}

output "worker_ips" {
  value = ["${aws_instance.worker_node.*.public_ip}"]
}

output "master_ips" {
  value = ["${aws_instance.master_node.*.public_ip}"]
}

output "self_host_etcd" {
  value = "${var.self_host_etcd}"
}

output "network_provider" {
  value = "${var.network_provider}"
}

output "calico_network_policy" {
  value = "${var.calico_network_policy}"
}
