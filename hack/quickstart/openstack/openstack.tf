resource "openstack_compute_secgroup_v2" "kubernetes_control_public" {
  name = "80AC0F3A-ECF8-424C-88FA-D8571F0C0EC9"
  description = "external controller security group"
  rule {
    from_port = 22
    to_port = 22
    ip_protocol = "tcp"
    cidr = "0.0.0.0/0"
  }
  rule {
    from_port = 443
    to_port = 443
    ip_protocol = "tcp"
    cidr = "0.0.0.0/0"
  }
}

resource "openstack_compute_secgroup_v2" "kubernetes_control_private" {
  name = "0C28F510-0569-4D47-BC90-710EB0111158"
  description = "internal controller security group"
  rule {
    from_port = 1
    to_port = 65535
    ip_protocol = "tcp"
    self = true
  }

}


resource "openstack_compute_secgroup_v2" "kubernetes_control_worker_private" {
  name = "1396C68D-8014-45AC-979B-A7EB4E6695F3"
  description = "internal controller worker security group"
  rule {
    from_port = 2379
    to_port = 2379
    ip_protocol = "tcp"
    from_group_id = "${openstack_compute_secgroup_v2.kubernetes_worker_private.id}"
  }
}

resource "openstack_compute_instance_v2" "control_node" {
  count = "${var.controller_count}"
  name = "control_node_${count.index}"
  image_id = "${var.image_id}"
  flavor_id = "${var.flavor_id}"
  key_pair = "${var.key_pair}"
  security_groups = ["${openstack_compute_secgroup_v2.kubernetes_control_public.name}", "${openstack_compute_secgroup_v2.kubernetes_control_private.name}", "${openstack_compute_secgroup_v2.kubernetes_control_worker_private.name}"]
  metadata {
    role = "controller"
  }
}


resource "openstack_compute_secgroup_v2" "kubernetes_worker_private" {
  name = "A4864BD5-7F82-46BD-81D9-0C4D51BFF1F9"
  description = "internal worker security group"
  rule {
    from_port = 1
    to_port = 65535
    ip_protocol = "tcp"
    self = true
  }

  rule {
    from_port = 1
    to_port = 65535
    ip_protocol = "tcp"
    from_group_id = "${openstack_compute_secgroup_v2.kubernetes_control_private.id}"
  }
}

resource "openstack_compute_secgroup_v2" "kubernetes_worker_public" {
  name = "49B7A860-FDD8-4A74-AE46-9AE954C26EBC"
  description = "external worker security group"
  rule {
    from_port = 30000
    to_port = 32000
    ip_protocol = "tcp"
    cidr = "0.0.0.0/0"
  }

  rule {
    from_port = 22
    to_port = 22
    ip_protocol = "tcp"
    cidr = "0.0.0.0/0"
  }
}


resource "openstack_compute_instance_v2" "worker_node" {
  count = "${var.worker_count}"
  name = "worker_node_${count.index}"
  image_id = "${var.image_id}"
  flavor_id = "${var.flavor_id}"
  key_pair = "${var.key_pair}"
  security_groups = ["${openstack_compute_secgroup_v2.kubernetes_worker_public.name}", "${openstack_compute_secgroup_v2.kubernetes_worker_private.name}"]
  metadata {
    role = "controller"
  }
}
