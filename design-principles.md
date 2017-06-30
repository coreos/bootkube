# Design Principles

There are exceptions to these principles, but these are general guidelines the project strives to adhere to.

## General

- Bootkube should be a single-use tool, which only runs on the first node in a cluster.
    - An exception is the `recovery` subcommand, discussed below.
- Bootkube should not be required to run to add new nodes to a cluster.
    - For example, adding nodes should not require a `bootkube join` command.
- Should not require flag or configuration coordination between the `render` and `start` steps.
    - Required flag coordination means certain `render` assets will only work with certain `start` flags, and this is something we should avoid.
    - For example, `bootkube render --self-hosted-etcd` requires no changes when ultimately running `bootkube start`.
- Avoid adding feature flags as much as possible. This makes testing & stability very difficult to maintain.
    - Users should be able to modify assets after `render` and before `start`. This allows flexibility for any use-case without bootkube owning the complexity in the codebase.
    - Complex rendering needs can be handled by custom rendering tools.
        - For example, the [CoreOS Tectonic Installer](https://github.com/coreos/tectonic-installer) performs its own rendering step, but utilizes `bootkube start` to launch the cluster.
- Launching compute resources is out of scope. Bootkube merely provides quickstart examples, but should not be perscriptive.

## Bootkube Render

- Bootkube is not meant to be a fully-featured rendering engine. There are much better tools for this - we shouldn't write yet another.
- Bootkube render should be considerd a useful starting point, which generates assets utilizing latest version and upstream best-practices.
- Should render assets for the most recent upstream release. If another version is desired, this can be left to the user to modify in their rendered templates.
- Should avoid using upstream alpha features, and instead allow a user to enable these on a case-by-case basis.
- Adding new configuration flags should be avoided!

## Bootkube Start

- Should be able to launch a cluster with only specifying an `--asset-dir`.
- Should be agnostic to the version of kubernetes cluster that is being launched.
- Should strive toward idempotent bootstrap operation.
    - Although, this is not currently the case when bootstrapping with self-hosted etcd.

## Bootkube Recovery

- When recovering with a functional API server, only a `kubeconfig` should be required.
- When recoverying from an etcd backup, no external assets should be required (all necessary data should be extracted from backup).
- The latest version of the recovery tool should strive to be able to recover all previously installed versions (no coupling between recovery process, and time of installation).

## Hack Directory

- Provides simple options for local development on bootkube
- Provide simple / non-production examples for some providers.
- Should not be perscriptive of how production compute should be launched or managed (e.g. Adding nodes, firewall rules, etc.)
