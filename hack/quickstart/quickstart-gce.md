## GCE Quickstart

### Choose a cluster prefix

This can be changed to identify separate clusters.

```
export CLUSTER_PREFIX=quickstart
```

### Add your SSH public key to project under user core

Quickstart scripts assume you can ssh into the `core` user. Either follow the commands below or go to the web console to add a key for the user `core`.

**WARNING** The following process will clobber existing keys in your project's ssh metadata.

Make a copy of your public key. It often exists at the path: `~/.ssh/id_rsa.pub`

Edit the copy of your public key to ensure it is prefixed with the username `core` The comment can be whatever you like. It should be in the following format:

```
core:ssh-rsa [KEY_VALUE] [KEY_COMMENT]
```

Then run the following command on the file:

```
$ gcloud compute project-info add-metadata --metadata-from-file sshKeys=[KEY_FILE_NAME].pub
```

Now any instances created on the project will by default have ssh access to the user core with your given ssh key unless project wide metatdata is specifically blocked on that instance.

### Launch Nodes

Launch nodes:

```
$ gcloud compute instances create ${CLUSTER_PREFIX}-core1 \
  --image-project coreos-cloud --image-family coreos-stable \
  --zone us-central1-a --machine-type n1-standard-1
```

Tag the first node as an apiserver node, and allow traffic to 443 on that node.

```
$ gcloud compute instances add-tags ${CLUSTER_PREFIX}-core1 --tags ${CLUSTER_PREFIX}-apiserver --zone us-central1-a
$ gcloud compute firewall-rules create ${CLUSTER_PREFIX}-443 --target-tags=${CLUSTER_PREFIX}-apiserver --allow tcp:443
```

### Bootstrap Master

*Replace* `<node-ip>` with the EXTERNAL_IP from output of `gcloud compute instances list ${CLUSTER_PREFIX}-core1`.

```
$ REMOTE_USER=$USER IDENT=~/.ssh/google_compute_engine ./init-master.sh <node-ip>
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
$ REMOTE_USER=$USER IDENT=~/.ssh/google_compute_engine ./init-node.sh <node-ip> cluster/auth/kubeconfig
```

**NOTE:** It can take a few minutes for each node to download all of the required assets / containers.
 They may not be immediately available, but the state can be inspected with:

```
$ kubectl --kubeconfig=cluster/auth/kubeconfig get nodes
```
