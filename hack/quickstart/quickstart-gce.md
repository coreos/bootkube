## GCE Quickstart

### Choose a cluster prefix

This can be changed to identify separate clusters.

```
export CLUSTER_PREFIX=quickstart
```

### Launch Nodes

Launch nodes:

```
$ gcloud compute instances create ${CLUSTER_PREFIX}-core1 \
  --image-project coreos-cloud --image-family coreos-stable \
  --zone us-central1-a --machine-type n1-standard-1
```

Tag the first node as an apiserver node, and allow traffic to 443 on that node.

```
$ gcloud compute instances add-tags ${CLUSTER_PREFIX}-core1 --tags ${CLUSTER_PREFIX}-apiserver
$ gcloud compute firewall-rules create ${CLUSTER_PREFIX}-443 --target-tags=${CLUSTER_PREFIX}-apiserver --allow tcp:443
```

### Bootstrap Master

*Replace* `<node-ip>` with the EXTERNAL_IP from output of `gcloud compute instances list k8s-core1`.

```
$ IDENT=~/.ssh/google_compute_engine ./init-master.sh <node-ip>
```

After the master bootstrap is complete, you can continue to add worker nodes. Or cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get nodes
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
$ IDENT=~/.ssh/google_compute_engine ./init-worker.sh <node-ip>
```

After a few minutes, time for the required assets and containers to be 
downloaded, the new worker will submit a Certificate Signing Request. This 
request must be approved for the worker to join the cluster. Until Kubernetes 
1.6, there is no [approve/deny] commands built in _kubectl_, therefore we must 
interact directly with the Kubernetes API. In the example below, we demonstrate
how the provided [csrctl.sh] tool can be used to manage CSRs.

```
$ ../csrctl.sh cluster/auth/admin-kubeconfig list
NAME        AGE       REQUESTOR           CONDITION
csr-9fxjw   16m       kubelet-bootstrap   Pending
csr-j9r05   22m       kubelet-bootstrap   Approved,Issued

$ ../csrctl.sh cluster/auth/admin-kubeconfig get csr-9fxjw
$ ../csrctl.sh cluster/auth/admin-kubeconfig approve csr-9fxjw

$ ../csrctl.sh cluster/auth/admin-kubeconfig list
NAME        AGE       REQUESTOR           CONDITION
csr-9fxjw   16m       kubelet-bootstrap   Approved,Issued
csr-j9r05   22m       kubelet-bootstrap   Approved,Issued
```

Once approved, the worker node should appear immediately in the node list:

```
$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get nodes
```

[approve/deny]: https://github.com/kubernetes/kubernetes/issues/30163
[csrctl.sh]: ../csrctl.sh