output "bootstrap_node_ip" {
  value = "${aws_instance.bootstrap_node.public_ip}"
}

output "worker_ips" {
  value = ["${aws_instance.worker_node.*.public_ip}"]
}

output "master_ips" {
  value = ["${aws_instance.master_node.*.public_ip}"]
}

output "network_provider" {
  value = "${var.network_provider}"
}
