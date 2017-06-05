# Terraform

This Terraform module generates bootkube assets, just like using the bootkube binary to run `bootkube render`. It aims to provide the same variables, defaults, features, and generated output that the command line tool does.

Terraform-based clusters can generate bootkube assets as part of `terraform apply`.

## TODO

Note, this module does not have feature parity with `bootkube render` yet.

* Experimental manifests
* etcd TLS
* Self-hosted etcd

## Usage

Use the terraform module within your terraform configs. Check `variables.tf` for required and optional input variables and `terraform.tfvars.example` for examples.

```hcl
module "bootkube" {
  source = "git://https://github.com/coreos/bootkube.git//terraform"

  cluster_name = "example"
  api_servers = ["node1.example.com"]
  etcd_servers = ["http://127.0.0.1:2379"]
  asset_dir = "/home/core/clusters/mycluster"
}
```

Alternately, copy `terraform.tfvars.example` to `terraform.tfvars` and run terraform commands directly from this module directory.

Render bootkube assets.

```sh
terraform get
terraform plan
terraform apply
```

### Comparison

Render bootkube assets directly with bootkube v0.4.2.

```sh
bootkube version
bootkube render --asset-dir=assets --api-servers=https://node1.example.com:443 --api-server-alt-names=DNS=node1.example.com --etcd-servers=http://127.0.0.1:2379
```

Compare assets. The only diffs you should see are TLS credentials.

```sh
diff -rw assets /home/core/cluster/mycluster
```
