# Node Agent

## Description

The Node Agent is a process that runs on each node of a Kubernetes cluster. The
purpose of this process is to monitor a specific annotation on the node object.
This annotation holds a JSON object which describes the version to be used for
the on-host kubelet.

This is useful for upgrading your on-host kubelet without ever having to ssh
into a machine. Simply update the annotation with the desired version, and the
and the Node Agent will ensure that the on-host kubelet matches that value,
and will restart the on-host kubelet so those new changes take effect.

## Usage

If you used bootkube to launch your cluster then the Node Agent will be
running by default.

The Node Agent looks for the annotation named:

```
"node-agent.alpha.coreos.com/desired-version"
```

To update the on-host kubelet, you can use the following steps:

1. Annotate the node you wish to update:

```
$ kubectl \
  --kubeconfig=cluster/auth/kubeconfig \
  annotate \
  --overwrite \
  node <your node> \
  node-agent.alpha.coreos.com/desired-version="v1.3.2_coreos.0"
```

1. Inspect the status of the on-host update by checking the following
annotation value:

```
"node-agent.alpha.coreos.com/current-version"
```

You can get the annotations on a Node via the following command:

```
$ kubectl get nodes -o=custom-columns=NAME:.metadata.name,ANNOTATIONS:.metadata.annotations
```

## More Detail

The Node Agent is responsible for updating a config file on disk that the kubelet
service file points to. Specifically, it will only modify the 
`KUBELET_VERSION` env var. Any other env vars in that file will be left
alone. After updating that file, the kubelet service is restarted via systemd.

