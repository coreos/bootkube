# Design Principles

There are exceptions to these principles, but these are general guidelines the project strives to adhere to.

## General

- Bootkube should be a single-use tool, which only runs on the first node in a cluster.
    - An exception is the `recover` subcommand, discussed below.
- Bootkube should not be required to add new nodes to an existing cluster.
    - For example, adding nodes should not require a `bootkube join` command.
    - Ideally all that should be required to add a node is starting the kubelet and providing a valid kubeconfig.
    - Configuration beyond the initial kubeconfig should be sourced from API objects. For example, the pod network is configured via CNI daemonset.
- Should not require flag or configuration coordination between the `render` and `start` steps.
    - Required flag coordination means certain `render` assets will only work with certain `start` flags, and this is something we should avoid.
- Avoid adding feature flags as much as possible. This makes testing & stability very difficult to maintain.
    - Users customize their cluster by modifying the output of `bootkube render` to fit their configuration. Users can generate their own assets for use with `bootkube start`, subject to following a small number of conventions.
    - Complex rendering needs can be handled by custom rendering tools.
        - For example, the [CoreOS Tectonic Installer](https://github.com/coreos/tectonic-installer) performs its own rendering step, but utilizes `bootkube start` to launch the cluster.
- Launching compute resources is out of scope. Bootkube merely provides quickstart examples, but should not be prescriptive.

## Bootkube Render

- Bootkube is not meant to be a fully-featured rendering engine. There are much better tools for this - we shouldn't write yet another.
- Bootkube render should be considered a useful starting point, which generates assets utilizing latest versions and best-practices.
- Should render assets for the most recent upstream release. If another version is desired, this can be left to the user to modify in their rendered templates.
- Should avoid using upstream alpha features, and instead allow a user to enable these on a case-by-case basis.
- Adding new configuration flags should be avoided. Instead users can customize the output and/or use an external rendering tool.

## Bootkube Start

- Should be able to launch a cluster by only specifying an `--asset-dir`.
- Should be agnostic to the version of kubernetes cluster that is being launched.
- Should strive toward idempotent bootstrap operation.
    - Although, this is not currently the case when bootstrapping with self-hosted etcd.

## Bootkube Recovery

- When running recovery, only a valid `kubeconfig` should be required (and in some cases may not be needed).
- The latest version of the recovery tool should strive to be able to recover all previously installed versions (no coupling between recovery process, and time of installation).

## Hack Directory

- Provides simple options for local development on bootkube
- Provide simple / non-production examples for some cloud providers.
- Should not be prescriptive of how production compute should be launched or managed (e.g. Adding nodes, firewall rules, etc.)
