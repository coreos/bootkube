## OpenStack Quickstart

These instructions have been tested on vexxhost.com. They should work for other providers though.

Ensure you have a working nova/glance/openstack tool that can authenticate before running these steps. Test this with `openstack image list` and `openstack server list`.

**Disclaimer**: This is "best effort maintained". This means that the instructions below are not actively maintained and may be out of date.
Please file a PR if you recognize any out-of-date instructions.

### Configuration

#### Find CoreOS Container Linux image

Find a CoreOS Container Linux image and use the ID as the value for the `image_id` terraform variable.

```
$ openstack image list | grep -i "container linux"
+--------------------------------------+-------------------------------------------+--------+
| ID                                   | Name                                      | Status |
+--------------------------------------+-------------------------------------------+--------+
...
| 90f57210-9354-4a2f-852e-d844237fbbad | Container Linux 1235.9.0                  | active |
...
+--------------------------------------+-------------------------------------------+--------+
```

Or import one into Glance using the [CoreOS Linux OpenStack instructions](https://coreos.com/os/docs/latest/booting-on-openstack.html#import-the-image).

#### Find a machine flavor

Find a machine flavor and use the ID as the value for the `flavor_id` terraform variable.

```
$ openstack flavor list
+--------------------------------------+-----------------+--------+------+-----------+-------+-----------+
| ID                                   | Name            |    RAM | Disk | Ephemeral | VCPUs | Is Public |
+--------------------------------------+-----------------+--------+------+-----------+-------+-----------+
...
| bbcb7eb5-5c8d-498f-9d7e-307c575d3566 | v1-standard-1   |   1024 |   40 |         0 |     2 | True      |
...
+--------------------------------------+-----------------+--------+------+-----------+-------+-----------+
```

### Cluster size

The terraform variable `controller_count` specifies the amount of the controller nodes.
The terraform variable `worker_count` specifies the amount of worker nodes.

Customize `openstack/config.tf` to add these image ids, ssh key, etc or use the terraform `-var` argument below to customize the configuration.

### Launch Nodes

Launch nodes using [Terraform](https://www.terraform.io/downloads.html):

This relies on the OpenStack credentials in the `OS_` environment variables used by the `glance` and `openstack` tool.

```
$ terraform apply \
    -var 'image_id=90f57210-9354-4a2f-852e-d844237fbbad' \
    -var 'flavor_id=bbcb7eb5-5c8d-498f-9d7e-307c575d3566' \
    openstack
```

After the master bootstrap is complete, cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get nodes
```

After a few minutes, time for the required assets and containers to be 
downloaded, the new worker will submit a Certificate Signing Request. This 
request must be approved for the worker to join the cluster. Until Kubernetes 
1.6, there is no [approve/deny] commands built in _kubectl_, therefore we must 
interact directly with the Kubernetes API. In the example below, we demonstrate
how the provided [csrctl.sh] tool can be used to manage CSRs.

```
$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get csr
NAME        AGE       REQUESTOR           CONDITION
csr-9fxjw   16m       kubelet-bootstrap   Pending
csr-j9r05   22m       kubelet-bootstrap   Approved,Issued

$ ../csrctl.sh cluster/auth/admin-kubeconfig approve csr-9fxjw
{
  "kind": "CertificateSigningRequest",
...
  "status": {
    "conditions": [
      {
        "type": "Approved",
        "lastUpdateTime": null
      }
    ]
  }
}

$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get csr
NAME        AGE       REQUESTOR           CONDITION
csr-9fxjw   16m       kubelet-bootstrap   Approved,Issued
csr-j9r05   22m       kubelet-bootstrap   Approved,Issued
```

Once approved, the worker node should appear immediately in the node list:

```
$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get nodes
```

### Cleanup

```
$ terraform destroy openstack/
```

### Owners

- Sergiusz Urbaniak <sur@coreos.com>
- Alex Somesan <alex.somesan@coreos.com>

[approve/deny]: https://github.com/kubernetes/kubernetes/issues/30163
[csrctl.sh]: ../csrctl.sh
