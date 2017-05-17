## Bootkube E2E Testing

This is the beginnings of E2E testing for the bootkube repo using the standard go testing harness. The framework package handles any inputs into the tests and expects that kubernetes cluster is already running. It is also a place to put common test code and patterns particularly around destructive testing interfaces that have yet to be created.

To run the tests once you have a kubeconfig to a running cluster just execute:
`go test -v --nodes=<number of nodes> --kubeconfig=<filepath> ./e2e/`

The number of nodes is required so that the framework can block on all nodes being registered.

## Writing tests

You can write tests more or less as you normally would in go except that at the beginning of every test one must call `framework.NewCluster()` to get access to the k8s client and any additional interfaces useful for testing. See `example_test.go` to get started

## Roadmap

The framework will need to be expanded to handle destructive testing such that all current tests in the pluton can be ported over.

## Requirements

Tests can't rely on network access to the cluster except via the kubernetes api.


