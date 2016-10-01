# Node Agent

## Description

The Update Controller is a process that runs within a Kubernetes cluster and is
responsible for updating or rolling back the cluster control plane. The update
controller works in conjunction with the [Node Agent](../node-agent/README.md).

## Usage

If you used bootkube to launch your cluster then the Update Controller will be
running by default. The update controller will sit in your cluster waiting to a
Third Party Resource known as the "Version" resource. This resource contains
information on the version each component in the cluster should be running.

A typical such resource would look like:

```json
{
   "metadata": {
     "name": "cluster-version"
   },
   "apiVersion": "update-controller.alpha.coreos.com/v1",
   "kind": "Version",
   "clusterVersion": "quay.io/coreos/hyperkube:vX.Y.Z",
}
```

### Components managed by the update controller

Each component in the cluster must opt-in to being managed by the update
controller. This opt-in process means that the component must have a label of
`update-controller-managed: true`. 

### Component priority

Each managed component must have a priority number. This number determins
which order the components get updated in. 

The priority is described via an annotation on the Kubernetes object:

```
update-controller.alpha.coreos.com/priority: "100"
```

By default, during an upgrade, the components are sorted in
ascending order and then updated. This ensures that certain components which
must be updated before others are handled properly.

### Rollbacks

Rollbacks are detected by the update controller when the new desired version is
less than the cluster version, which is the highest versioned component that is
being managed by the controller. When a rollback is detected, all components are
sorted in descending order, and then rolled back to the new desired version.

