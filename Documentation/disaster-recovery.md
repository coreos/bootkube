# Disaster Recovery

Self-hosted Kubernetes clusters are vulnerable to the following catastrophic
failure scenarios:

- Loss of all api-servers
- Loss of all schedulers
- Loss of all controller-managers
- Loss of all self-hosted etcd nodes

To minimize the likelihood of any of the these scenarios, production
self-hosted clusters should always run in a [high-availability
configuration](https://kubernetes.io/docs/admin/high-availability/).

Nevertheless, in the event of a control plane loss the bootkube project
provides limited disaster avoidance and recovery support through the
`pod-checkpointer` program and the `bootkube recover` subcommand.

## Pod Checkpointer

The Pod Checkpointer is a program that ensures that existing local pod state
can be recovered in the absence of an api-server.

This is accomplished by managing "checkpoints" of local pod state as static pod
manifests:

- When the checkpointer sees that a "parent pod" (a pod which should be
  checkpointed), is successfully running, the checkpointer will save a local
  copy of the manifest.
- If the parent pod is detected as no longer running, the checkpointer will
  "activate" the checkpoint manifest. It will allow the checkpoint to continue
  running until the parent-pod is restarted on the local node, or it is able to
  contact an api-server to determine that the parent pod is no longer scheduled
  to this node.

A Pod Checkpointer DaemonSet is deployed by default when using `bootkube
render` to create cluster manifests. Using the Pod Checkpointer is highly
recommended for all self-hosted clusters to ensure node reboot resiliency.

For more information, see the [Pod Checkpointer
README](https://github.com/kubernetes-incubator/bootkube/blob/master/cmd/checkpoint/README.md).

## Bootkube Recover

In the event of partial or total self-hosted control plane loss, `bootkube
recover` may be able to assist in re-bootstrapping the self-hosted control
plane.

The `bootkube recover` subcommand does not recover a cluster directly. Instead,
it extracts the control plane configuration from an available source and
renders manifests in a format that `bootkube start` can use invoked to reboot
the cluster.

To see available options, run:

```
bootkube recover --help
```

To recover a cluster, first invoke `bootkube recover` with flags corresponding
to the current state of the cluster (supported states listed below). Then,
invoke `bootkube start` to reboot the cluster. For example:

```
scp bootkube user@master-node:
ssh user@master-node
./bootkube recover --asset-dir=recovered [scenario-specific options]
sudo ./bootkube start --asset-dir=recovered
```

For complete recovery examples see the
[hack/multi-node/bootkube-test-recovery](https://github.com/kubernetes-incubator/bootkube/blob/master/hack/multi-node/bootkube-test-recovery)
and
[hack/multi-node/bootkube-test-recovery-self-hosted-etcd](https://github.com/kubernetes-incubator/bootkube/blob/master/hack/multi-node/bootkube-test-recovery-self-hosted-etcd)
scripts. The `bootkube-test-recovery` script is demoed below.

[![asciicast](https://asciinema.org/a/dsp43ziuuzwcztni94y8l25s5.png)](https://asciinema.org/a/dsp43ziuuzwcztni94y8l25s5)

### If an api-server is still running

If an api-server is still running but other control plane components are down,
preventing cluster functionality (i.e. the scheduler pods are all down), the
control plane can be extracted directly from the api-server:

```
bootkube recover --asset-dir=recovered --kubeconfig=/etc/kubernetes/kubeconfig
```
### If an external etcd cluster is still running

If using an external (non-self-hosted) etcd cluster, the control plane can be
extracted directly from etcd:

```
bootkube recover --asset-dir=recovered --etcd-servers=http://127.0.0.1:2379 --kubeconfig=/etc/kubernetes/kubeconfig
```

### If an etcd backup is available (non-self-hosted etcd)

First, recover the external etcd cluster from the backup. Then use the method
described in the previous section to recover the control plane manifests.

### If an etcd backup is available (self-hosted etcd)

If using self-hosted etcd, recovery is supported via reading from an etcd
backup file:

```
bootkube recover --asset-dir=recovered --etcd-backup-file=backup --kubeconfig=/etc/kubernetes/kubeconfig
```

In addition to rebooting the control plane, this will also destroy and recreate
the self-hosted etcd cluster using the backup.
