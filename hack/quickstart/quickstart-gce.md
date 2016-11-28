## GCE Quickstart

### Choose a cluster prefix

This can be changed to identify separate clusters.

```
export CLUSTER_PREFIX=quickstart
```

### Launch Nodes

To find the latest CoreOS alpha/beta/stable images, please see the [CoreOS GCE Documentation](https://coreos.com/os/docs/latest/booting-on-google-compute-engine.html). Then replace the `--image` flag in the command below.

Launch nodes:

```
$ gcloud compute instances create ${CLUSTER_PREFIX}-core1 \
  --image https://www.googleapis.com/compute/v1/projects/coreos-cloud/global/images/coreos-stable-1068-9-0-v20160809 \
  --zone us-central1-a --machine-type n1-standard-1
```

Tag the first node as an apiserver node, and allow traffic to 443 on that node.

```
$ gcloud compute instances add-tags ${CLUSTER_PREFIX}-core1 --tags ${CLUSTER_PREFIX}-apiserver
$ gcloud compute firewall-rules create ${CLUSTER_PREFIX}-443 --target-tags= ${CLUSTER_PREFIX}-apiserver --allow tcp:443
```

### Bootstrap Master

*Replace* `<node-ip>` with the EXTERNAL_IP from output of `gcloud compute instances list k8s-core1`.

```
$ IDENT=~/.ssh/google_compute_engine ./init-master.sh <node-ip>
```

After the master bootstrap is complete, you can continue to add worker nodes. Or cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```

### Add Workers

Run the `Launch Nodes` step for each additional node you wish to add (changing the name from ` ${CLUSTER_PREFIX}-core1`)

Get the EXTERNAL_IP from each node you wish to add:

```
$ gcloud compute instances list ${CLUSTER_PREFIX}-core2
$ gcloud compute instances list ${CLUSTER_PREFIX}-core3
```

Initialize each worker node by replacing `<node-ip>` with the EXTERNAL_IP from the commands above.

```
$ IDENT=~/.ssh/google_compute_engine ./init-worker.sh <node-ip> cluster/auth/kubeconfig
```

**NOTE:** It can take a few minutes for each node to download all of the required assets / containers.
 They may not be immediately available, but the state can be inspected with:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```
