# ssh

resource "tls_private_key" "core" {
  algorithm = "RSA"
}

resource "null_resource" "export" {
  provisioner "local-exec" {
    command = "echo '${tls_private_key.core.private_key_pem}' >id_rsa_core && chmod 0600 id_rsa_core"
  }

  provisioner "local-exec" {
    command = "echo '${tls_private_key.core.public_key_openssh}' >id_rsa_core.pub"
  }
}

resource "openstack_compute_keypair_v2" "k8s_keypair" {
  name       = "bootkube_keypair"
  public_key = "${tls_private_key.core.public_key_openssh}"
}

# compute instances

resource "openstack_compute_instance_v2" "control_node" {
  count       = "${var.controller_count}"
  name        = "bootkube_control_node_${count.index}"
  image_id    = "${var.image_id}"
  flavor_id   = "${var.flavor_id}"
  key_pair    = "${openstack_compute_keypair_v2.k8s_keypair.name}"

  metadata {
    role = "controller"
  }

  network {
    name = "public"
  }

  connection {
    user        = "core"
    private_key = "${tls_private_key.core.private_key_pem}"
  }

  provisioner "file" {
    source      = "kubelet.master"
    destination = "/home/core/kubelet.master"
  }

  provisioner "file" {
    source      = "init-master.sh"
    destination = "/home/core/init-master.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo mv /home/core/kubelet.master /etc/systemd/system/kubelet.service",
      "chmod +x /home/core/init-master.sh",
      "sudo /home/core/init-master.sh local"
    ]
  }

  # https://github.com/hashicorp/terraform/issues/3379
  provisioner "local-exec" {
    command = "mkdir ${var.cluster_dir} && scp -o StrictHostKeyChecking=no -q -i id_rsa_core -r core@${self.access_ip_v4}:/home/core/assets/* ${var.cluster_dir}"
  }
}

resource "openstack_compute_instance_v2" "worker_node" {
  count     = "${var.worker_count}"
  name      = "bootkube_worker_node_${count.index}"
  image_id  = "${var.image_id}"
  flavor_id = "${var.flavor_id}"
  key_pair  = "${openstack_compute_keypair_v2.k8s_keypair.name}"

  metadata {
    role = "worker"
  }

  network {
    name = "public"
  }

  depends_on = ["openstack_compute_instance_v2.control_node"]

  connection {
    user = "core"
    private_key = "${tls_private_key.core.private_key_pem}"
  }

  provisioner "file" {
    source      = "kubelet.worker"
    destination = "/home/core/kubelet.worker"
  }

  provisioner "file" {
    source      = "init-worker.sh"
    destination = "/home/core/init-worker.sh"
  }

  provisioner "file" {
    source      = "${var.cluster_dir}/auth/bootstrap-kubeconfig"
    destination = "/home/core/bootstrap-kubeconfig"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /home/core/init-worker.sh",
      "sudo /home/core/init-worker.sh local"
    ]
  }
}
