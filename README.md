# Bootkube

Bootkube is a helper tool for launching self-hosted Kubernetes clusters.

When launched, bootkube will act as a temporary Kubernetes control-plane (api-server, scheduler, controller-manager), which operates long enough to bootstrap a replacement self-hosted control-plane.

Additionally, bootkube can be used to generate all of the necessary assets for use in bootstrapping a new cluster. These assets can then be modified to support any additional configuration options.

If you are interested in the design and details [see the Kubernetes self-hosted design document](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/self-hosted-kubernetes.md) which provides an architecture overview.

## Guides

* [GCE Quickstart](hack/quickstart/quickstart-gce.md)
* [AWS Quickstart](hack/quickstart/quickstart-aws.md)
* [CoreOS Baremetal](https://github.com/coreos/coreos-baremetal/blob/master/Documentation/bootkube.md)
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

In the case of a partial or total control plane outage (i.e. due to lost master nodes) an experimental `recover` command can extract and write manifests from a backup location. These manifests can then be used by the `start` command to reboot the cluster. Currently recovery from a running apiserver or external running etcd cluster are the only supported methods.

To see available options, run:

```
bootkube recover --help
```

Recover from an external running etcd cluster:

```
bootkube recover --asset-dir=recovered --etcd-servers=http://127.0.0.1:2379 --kubeconfig=/etc/kubernetes/kubeconfig
```

Recover from a running apiserver (i.e. if the scheduler pods are all down):

```
bootkube recover --asset-dir=recovered --kubeconfig=/etc/kubernetes/kubeconfig
```

For a complete recovery example please see the [hack/multi-node/bootkube-test-recovery](hack/multi-node/bootkube-test-recovery) script.

## Building

First, clone the repo into the proper location in your $GOPATH:

```
go get -u github.com/kubernetes-incubator/bootkube
cd $GOPATH/src/github.com/kubernetes-incubator/bootkube
```

Install Golang 1.7.x, seems to be issues with 1.6.3 on ubuntu 16.04

```
sudo apt-get update
sudo apt-get -y upgrade
sudo wget https://storage.googleapis.com/golang/go1.7.4.linux-amd64.tar.gz
tar -xvf go1.7.4.linux-amd64.tar.gz
sudo mv go /usr/local

export GOROOT=/usr/local/go
export GOPATH=$HOME/gocode
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

Add export line abobe to ~./profile to survive reboot
```
Verify go version

```
# go version
go version go1.7.4 linux/amd64
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
The basic test suite should run automatically on PRs. It can be re-run by commenting to the PR: `rktbot run tests`. To whitelist an external contributor's PR temporarily, one can use the `ok to test` command.

More extensive self-hosted etcd testing can be run by commenting: `rktbot run etcd tests`. The maintainers will be responsible for determining if this is needed and should pass as it may contain experimental or flaky tests.

If changes are made to the checkpointer code you must run a third type of test: `rktbot run checkpointer tests`

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
