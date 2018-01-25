provider "aws" {
  access_key = "${var.access_key_id}"
  secret_key = "${var.access_key}"
  region     = "${var.region}"
  version    = "1.8"
}

resource "aws_key_pair" "core" {
  key_name   = "${var.resource_owner}"
  public_key = "${var.ssh_public_key}"
}

resource "aws_instance" "bootstrap_node" {
  ami                  = "${data.aws_ami.coreos_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${aws_key_pair.core.key_name}"
  iam_instance_profile = "${aws_iam_instance_profile.bk_profile.id}"

  vpc_security_group_ids      = ["${aws_security_group.allow_all.id}"]
  subnet_id                   = "${aws_subnet.main.id}"
  associate_public_ip_address = true
  depends_on                  = ["aws_internet_gateway.main"]

  tags {
    Name = "${var.resource_owner}"
  }

  root_block_device {
    volume_type = "gp2"
    volume_size = "30"
  }

  provisioner "file" {
    source      = "environment_${var.environment}.txt"
    destination = "/tmp/environment"

    connection {
      user = "core"
    }
  }

  provisioner "remote-exec" {
    # coreos manages /etc/environment, so append to the file
    inline = [
      "sudo bash -c 'cat /tmp/environment >> /etc/environment'",
      "sudo rm -f /tmp/environment",
    ]

    connection {
      user = "core"
    }
  }
}

resource "aws_instance" "worker_node" {
  ami                  = "${data.aws_ami.coreos_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${aws_key_pair.core.key_name}"
  count                = "${var.num_workers}"
  iam_instance_profile = "${aws_iam_instance_profile.bk_profile.id}"

  vpc_security_group_ids      = ["${aws_security_group.allow_all.id}"]
  subnet_id                   = "${aws_subnet.main.id}"
  associate_public_ip_address = true
  depends_on                  = ["aws_internet_gateway.main"]

  tags {
    Name = "${var.resource_owner}"
  }

  root_block_device {
    volume_type = "gp2"
    volume_size = "30"
  }

  provisioner "file" {
    source      = "environment_${var.environment}.txt"
    destination = "/tmp/environment"

    connection {
      user = "core"
    }
  }

  provisioner "remote-exec" {
    # coreos manages /etc/environment, so append to the file
    inline = [
      "sudo bash -c 'cat /tmp/environment >> /etc/environment'",
      "sudo rm -f /tmp/environment",
    ]

    connection {
      user = "core"
    }
  }
}

resource "aws_instance" "master_node" {
  ami                  = "${data.aws_ami.coreos_ami.image_id}"
  instance_type        = "${var.instance_type}"
  key_name             = "${aws_key_pair.core.key_name}"
  count                = "${var.additional_masters}"
  iam_instance_profile = "${aws_iam_instance_profile.bk_profile.id}"

  vpc_security_group_ids      = ["${aws_security_group.allow_all.id}"]
  subnet_id                   = "${aws_subnet.main.id}"
  associate_public_ip_address = true
  depends_on                  = ["aws_internet_gateway.main"]

  tags {
    Name = "${var.resource_owner}"
  }

  root_block_device {
    volume_type = "gp2"
    volume_size = "30"
  }

  provisioner "file" {
    source      = "environment_${var.environment}.txt"
    destination = "/tmp/environment"

    connection {
      user = "core"
    }
  }

  provisioner "remote-exec" {
    # coreos manages /etc/environment, so append to the file
    inline = [
      "sudo bash -c 'cat /tmp/environment >> /etc/environment'",
      "sudo rm -f /tmp/environment",
    ]

    connection {
      user = "core"
    }
  }
}

data "aws_ami" "coreos_ami" {
  most_recent = true

  filter {
    name   = "name"
    values = ["CoreOS-stable-*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "owner-id"
    values = ["595879546273"]
  }
}
