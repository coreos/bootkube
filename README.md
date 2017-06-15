# Bootkube

Bootkube is a helper tool for launching self-hosted Kubernetes clusters.

When launched, bootkube will act as a temporary Kubernetes control-plane (api-server, scheduler, controller-manager), which operates long enough to bootstrap a replacement self-hosted control-plane.

Additionally, bootkube can be used to generate all of the necessary assets for use in bootstrapping a new cluster. These assets can then be modified to support any additional configuration options.

If you are interested in the design and details [see the Kubernetes self-hosted design document](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/self-hosted-kubernetes.md) which provides an architecture overview.

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

### Recover a downed cluster

In the case of a partial or total control plane outage (i.e. due to lost master nodes) an experimental `recover` command can extract and write manifests from a backup location. These manifests can then be used by the `start` command to reboot the cluster. Currently recovery from a running apiserver, an external running etcd cluster, or an etcd backup taken from the self hosted etcd cluster are the methods.

For more details and examples see [disaster recovery documentation](Documentation/disaster-recovery.md).

## Building

First, clone the repo into the proper location in your $GOPATH:

```
go get -u github.com/kubernetes-incubator/bootkube
cd $GOPATH/src/github.com/kubernetes-incubator/bootkube
```

Then, to build:

```
make
```

And optionally, to install into $GOPATH/bin:

```
make install
```

### Development
Running `make run-single` or `make run-multi` resets the single-node or multi-node Vagrant environments, respectively, in the background while compiling bootkube and provisions the fresh VM with bootkube when compilation is complete. This is intended to speed up iteration.

## Running PR Tests
The basic test suite should run automatically on PRs. It can be re-run by commenting to the PR: `coreosbot run e2e`. To whitelist an external contributor's PR temporarily, one can use the `ok to test` command.

If changes are made to the checkpointer code you manually trigger tests to run via: `coreosbot run e2e checkpointer`. This builds the checkpointer image from the repo and includes that image into the test bootkube binary being built. All the same tests are run.

## Running conformace tests on PRs
Commenting `coreosbot run conformance` will trigger conformance tests against that PR.

## Conformance Tests

This repository includes scripts for running the Kubernetes conformance tests agains the [hack/single-node](hack/single-node) and [hack/multi-node](hack/multi-node) launched clusters.

To run the conformance tests:

```
make conformance-single
```

or

```
make conformance-multi
```

## Roadmap

- Document, test, and create best practice around disaster recovery
- Work upstream to resolve the long-term self-hosted bootstrapping
- Self-host other components like etcd and networking to prove the model
- Get the concept of kubelet checkpointing upstreamed
- Prove out the self-hosted model on all new releases of k8s and different platforms
- Document best-practices for upgrades and upstream the idea of a upgradable daemonset

## Getting Involved

Want to contribute to bootkube? Have Questions? We are looking for active participation from the community

You can find us at the bootkube channel on [Kubernetes slack](https://github.com/kubernetes/community#slack-chat)

## License

bootkube is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
