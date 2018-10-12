# Bootkube  

[![Build Status](https://travis-ci.org/kubernetes-incubator/bootkube.svg?branch=master)](https://travis-ci.org/kubernetes-incubator/bootkube)
[![GoDoc](https://godoc.org/github.com/kubernetes-incubator/bootkube?status.svg)](https://godoc.org/github.com/kubernetes-incubator/bootkube)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/bootkube)](https://goreportcard.com/report/github.com/kubernetes-incubator/bootkube)

Bootkube is a tool for launching self-hosted Kubernetes clusters.

When launched, bootkube will deploy a temporary Kubernetes control-plane (api-server, scheduler, controller-manager), which operates long enough to bootstrap a replacement self-hosted control-plane.

Additionally, bootkube can be used to generate all of the necessary assets for use in bootstrapping a new cluster. These assets can then be modified to support any additional configuration options.

## Details of self-hosting

* [KubeCon self-hosted presentation video](https://www.youtube.com/watch?v=EbNxGK9MwN4)
* [Kubernetes self-hosted design document](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/cluster-lifecycle/self-hosted-kubernetes.md)

## Guides

* [GCE Quickstart](hack/quickstart/quickstart-gce.md)
* [AWS Quickstart](hack/quickstart/quickstart-aws.md)
* [Bare-Metal](https://github.com/coreos/matchbox/tree/master/examples/terraform/bootkube-install)
* [Vagrant Single-Node](hack/single-node/README.md)
* [Vagrant Multi-Node](hack/multi-node/README.md)

## Usage

Bootkube has two main commands: `render` and `start`.

There is a third, experimental command `recover` which can help reboot a downed cluster (see below).

### Render assets

Bootkube can be used to render all of the assets necessary for bootstrapping a self-hosted Kubernetes cluster. This includes generation of TLS assets, Kubernetes object manifests, and a kubeconfig to connect to the bootstrapped cluster.

To see available options, run:

```
bootkube render --help
```

Example:

```
bootkube render --asset-dir=my-cluster
```

The resulting assets can be inspected / modified in the generated asset-dir.

### Start bootkube

To start bootkube use the `start` subcommand.

To see available options, run:

```
bootkube start --help
```

Example:

```
bootkube start --asset-dir=my-cluster
```

When `bootkube start` is creating Kuberentes resources from manifests, the following order is used:
1. Any `Namespace` objects are created, in lexicographical order.
1. Any `CustomResourceDefinition` objects are created, in lexicographical order.
1. Any remaining resouces are created, in lexicographical order.

### Recover a downed cluster

In the case of a partial or total control plane outage (i.e. due to lost master nodes) an experimental `recover` command can extract and write manifests from a backup location. These manifests can then be used by the `start` command to reboot the cluster. Currently recovery from a running apiserver, an external running etcd cluster, or an etcd backup taken from the self hosted etcd cluster are the methods.

For more details and examples see [disaster recovery documentation](Documentation/disaster-recovery.md).

## Development

See [Documentation/development.md](Documentation/development.md) for more information.

## Getting Involved

Want to contribute to bootkube? Have Questions? We are looking for active participation from the community

You can find us at the `#bootkube` channel on [Kubernetes slack][slack].

## Related Links

* [Users and Integrations](Documentation/users-integrations.md)

## License

bootkube is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.

[slack]: https://github.com/kubernetes/community/tree/master/communication#social-media
testing builders
