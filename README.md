# Bootkube

Bootkube is a helper tool for launching self-hosted Kubernetes clusters.

When launched, bootkube will act as a temporary Kubernetes control-plane (api-server, scheduler, controller-manager), which operates long enough to bootstrap a replacement self-hosted control-plane.

Additionally, bootkube can be used to generate all of the necessary assets for use in bootstrapping a new cluster. These assets can then be modified to support any additional configuration options.

If you are interested in the design and details [see the bootkube design document](https://docs.google.com/document/d/1VNp4CMjPPHevh2_JQGMl-hpz9JSLq3s7HlI87CTjl-8/edit).

## Guides

* [GCE Quickstart](hack/quickstart/quickstart-gce.md)
* [AWS Quickstart](hack/quickstart/quickstart-aws.md)
* [CoreOS Baremetal](https://github.com/coreos/coreos-baremetal/blob/master/Documentation/bootkube.md)
* [Vagrant Single-Node](hack/single-node/README.md)
* [Vagrant Multi-Node](hack/multi-node/README.md)
* [corectl Multi-Node](hack/corectl/README.md)

## Usage

Bootkube has two main commands: `render` and `start`

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

## Building

First, clone the repo into the proper location in your $GOPATH:

```
go get -u github.com/coreos/bootkube
cd $GOPATH/src/github.com/coreos/bootkube
```

Then, to build:

```
make
```

And optionally, to install into $GOPATH/bin:

```
make install
```

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

## License

bootkube is under the Apache 2.0 license. See the [LICENSE](LICENSE) file for details.
