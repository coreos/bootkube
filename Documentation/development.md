# Bootkube Development

## Requirements

* Go 1.8+
* Configured [GOPATH](http://golang.org/doc/code.html#GOPATH)

## Building

First, clone the repo into the proper location in your $GOPATH:

```
go get -u github.com/kubernetes-incubator/bootkube
cd $GOPATH/src/github.com/kubernetes-incubator/bootkube
```

Then, to build (only Go verson 1.8 is supported now):

```
make clean all
```

## Local Development Environments

To easily launch local vagrant development clusters:

```
# Launch a single-node cluster
make run-single
```

```
# Launch a multi-node cluster
make run-multi
```

Each of these commands will recompile bootkube, then render new assets and provision a new cluster.

Additionally, if you wish to run upstream Kubernetes conformance tests against these local clusters:

```
make conformance-single
```

```
make conformance-multi
```

## Running PR Tests

The basic test suite should run automatically on PRs, but can also be triggered manually.

Commenting on the PR:

* `ok to test`: whitelists an external contributor's PR as safe to test.
* `coreosbot run e2e`: re-runs the end-to-end test suite.
* `coreosbot run e2e checkpointer`: can be used to specifically test new checkpointer code.
    * This will build a new checkpointer image from the PR, and includes that image as part of the checkpointer daemonset.
* `coreosbot run conformance`: run upstream Kubernetes conformance tests
