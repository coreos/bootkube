## AWS Quickstart

### Choose a cluster prefix

This can be changed to identify separate clusters.

```
export CLUSTER_PREFIX=quickstart
```

### Configure Security Groups

Make note of the `GroupId` output of this command, as it will be referenced later in this guide as `<K8S_SG_ID>`.

```
$ aws ec2 create-security-group --region us-west-2 --group-name ${CLUSTER_PREFIX}-sg --description "Security group for ${CLUSTER_PREFIX} cluster"
GroupID: "sg-abcdefg"
```

Next, create the security group rules.

```
$ aws ec2 authorize-security-group-ingress --region us-west-2 --group-name ${CLUSTER_PREFIX}-sg --protocol tcp --port 22 --cidr 0.0.0.0/0
$ aws ec2 authorize-security-group-ingress --region us-west-2 --group-name ${CLUSTER_PREFIX}-sg --protocol tcp --port 443 --cidr 0.0.0.0/0
$ aws ec2 authorize-security-group-ingress --region us-west-2 --group-name ${CLUSTER_PREFIX}-sg --protocol tcp --port 0-65535 --source-group k8s-sg
```

### Create a key-pair

```
$ aws ec2 create-key-pair --key-name ${CLUSTER_PREFIX}-key --query 'KeyMaterial' --output text > ${CLUSTER_PREFIX}-key.pem
$ chmod 400 ${CLUSTER_PREFIX}-key.pem
```

### Launch Nodes

To find the latest CoreOS alpha/beta/stable images, please see the [CoreOS AWS Documentation](https://coreos.com/os/docs/latest/booting-on-ec2.html). Then replace the `--image-id` in the command below.

In the command below, replace `<K8S_SG_ID>` with the security-group-id noted earlier.

```
$ aws ec2 run-instances --region us-west-2 --image-id ami-184a8f78 --security-group-ids <K8S_SG_ID> --count 1 --instance-type m3.medium --key-name ${CLUSTER_PREFIX}-key --query 'Instances[0].InstanceId'
"i-abcdefgh"
```

Next we will use the output of the above command (instance-id) in place of <INSTANCE_ID> in the command below:

```
$ aws ec2 describe-instances --region us-west-2 --instance-ids <INSTANCE_ID> --query 'Reservations[0].Instances[0].PublicIpAddress'
```

### Bootstrap Master

We can then use the public-ip to initialize a master node:

```
$ IDENT=k8s-key.pem ./init-master.sh <PUBLIC_IP>
```

After the master bootstrap is complete, you can continue to add worker nodes. Or cluster state can be inspected via kubectl:

```
$ kubectl --kubeconfig=cluster/auth/admin-kubeconfig get nodes
```

### Add Workers

Run the `Launch Nodes` step for each additional node you wish to add, then using the public-ip, run:

```
IDENT=k8s-key.pem ./init-worker.sh <PUBLIC_IP>
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