# Hack / Dev multi-node build

**Note: All scripts are assumed to be ran from this directory.**

## Quickstart

This will generate the default assets in the `cluster` directory and launch multi-node self-hosted cluster.

```
./bootkube-up
```

Configure Kubeconfig env variable allow using kubectl without paramaters

```
export KUBECONFIG=$GOPATH/src/github.com/kubernetes-incubator/bootkube/hack/multi-node/cluster/auth/kubeconfig
```

Verify kubctl

```
kubectl get pods -o wide --all-namespaces -w
NAMESPACE     NAME                   READY     STATUS    RESTARTS   AGE       IP             NODE
kube-system   kube-apiserver-83hf8   1/1       Running   2          1h        172.17.4.101   172.17.4.101
kube-system   kube-controller-manager-3170159-48pfn   1/1       Running   1         1h        10.2.1.6   172.17.4.101
...
```

## Cleaning up

To stop the running cluster and remove generated assets, run:

```
vagrant destroy -f
rm -rf cluster
```
