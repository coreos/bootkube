## OpenStack Quickstart

These instructions have been tested on vexxhost.com. They should work for other providers though.

Ensure you have a working nova/glance/openstack tool that can authenticate before running these steps. Test this with `glance image-list` and `openstack server list`.

### Find CoreOS Linux image

Find a CoreOS Linux image

```
glance image-list | grep -i 'coreos'
```

Or import one into Glance using the [CoreOS Linux OpenStack instructions](https://coreos.com/os/docs/latest/booting-on-openstack.html#import-the-image).

Customize `openstack/config.tf` to add these image ids, ssh key, etc.

### Launch Nodes

Launch nodes using [Terraform](https://www.terraform.io/downloads.html):

This relies on the OpenStack credentials in the `OS_` environment variables used by the `glance` and `openstack` tool.

```
$ terraform apply openstack/
```


### Bootstrap Master

*Replace* `<node-ip>` with the EXTERNAL_IP from output of `openstack server list --format=json --name 'control_node_.' | jq .[].Networks`.

```
export IDENT=~/.ssh/id_rsa.pub; ./init-master.sh <node-ip>
```

After the master bootstrap is complete, you can continue to add worker nodes. Or cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

### Add Workers

*Replace* `<node-ip>` with the EXTERNAL_IP from output of `openstack server list --format=json --name 'worker_node_.' | jq .[].Networks`.

Initialize each worker node by replacing `<node-ip>` with the EXTERNAL_IP from the commands above.

```
$ IDENT=~/.ssh/google_compute_engine ./init-worker.sh <node-ip> cluster/auth/kubeconfig
```

**NOTE:** It can take a few minutes for each node to download all of the required assets / containers.
 They may not be immediately available, but the state can be inspected with:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

### Cloud Integration

TODO: Automate this

```
$ cat /etc/kubernetes/cloud.conf
[Global]
auth-url=https://auth.vexxhost.net/v2.0
username=
password=
region=ca-ymq-1
tenant-name=
```

Add the following flags to `/etc/systemd/system/kubelet.service`

```
--cloud-provider=openstack  \
--cloud-config=/etc/kubernetes/cloud.conf \
```

Similarly on the controller manager:

```
kubectl edit deployment -n kube-system kube-controller-manager
```

```
      volumes:
      - hostPath:
          path: /etc/kubernetes
        name: kubernetes
```

```
    spec:
      containers:
      - command:
        - ./hyperkube
        - controller-manager
        - --cloud-config=/etc/kubernetes/host/cloud.conf
        - --cloud-provider=openstack
```


### Cleanup

```
$ terraform destroy openstack/
```