# Node Agent

## Description

The Node Agent is a process that runs on each node of a Kubernetes cluster. The
purpose of this process is to monitor a specific annotation on the node object.
This annotation holds a JSON object which describes the version and configuration
to be used for the on-host kubelet.

This is useful for upgrading your on-host kubelet without ever having to ssh
into a machine. Simply update the annotation with the desired version, and the
name of the configMap that holds your configuration, and the Node Agent will
ensure that the on-host kubelet matches those values, and will restart the on-host
kubelet so those new changes take effect.

## Usage

If you are using a supported self-hosted cluster then the Node Agent will be
running by default.

The Node Agent looks for the annotation named:

```
"node-agent.alpha.coreos.com/desired-config"
```

The format of the JSON object used by the Node Agent is:

```json
{
        "KUBELET_CONFIG": "<your configMap name>",
        "KUBELET_VERSION": "<desired kubelet version>",
        "KUBELET_OPTS": "<kubelet flags>"
}
```

To update the on-host kubelet, you can use the following steps:

1. If this is your first time upgrading the kubelet, you will need to create a
ConfigMap object. By default, the on-host and self-hosted kubelets get their
configuration from an on-disk "checkpointed" configuration string. An example
ConfigMap object might look like:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: kubelet-config
  namespace: kube-system
data:
  kubelet-flags: "--api-servers='https:172.17.0.100' --config=/etc/kubernetes/manifests --allow-privileged --cluster-dns=10.3.0.10 --cluster-domain=cluster.local --kubeconfig=/etc/kubernetes/kubeconfig --lock-file=/var/run/lock/kubelet.lock"
```

You can get the current flags the kubelet was configured with by looking at
the `node-agent.alpha.coreos.com/current-config` annotation on the desired node.
Scroll down for an example of how to get that annotation value.

2. Annotate the node you wish to update:

```
$ kubectl \
  --kubeconfig=cluster/auth/kubeconfig \
  annotate \
  --overwrite \
  node <your node> \
  node-agent.alpha.coreos.com/desired-config='{"KUBELET_VERSION": "v1.3.2_coreos.0", "KUBELET_CONFIG": "kubelet-config-v2"}'
```

3. Inspect the status of the on-host update by checking the following
annotation value:

```
"node-agent.alpha.coreos.com/current-config"
```

You can get the annotations on a Node via the following command:

```
$ kubectl get nodes -o=custom-columns=NAME:.metadata.name,ANNOTATIONS:.metadata.annotations
```

## More Detail

The Node Agent is responsible for updating a config file on disk that the kubelet
service file points to. Specifically, it will only modify the `$KUBELET_OPTS`
and `KUBELET_VERSION` env vars. Any other env vars in that file will be left
alone. After updating that file, the kubelet service is restarted via systemd.
