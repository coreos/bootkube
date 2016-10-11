# Hack / Dev multi-node build

## [corectl](github.com/TheNewNormal/corectl) is currently macOS specific

**Note: All scripts are assumed to be ran from this directory.**

## Quickstart

- Setup [corectl](github.com/TheNewNormal/corectl).
- Make sure that `corectld` is up and running

```
$ corectld start
```


This will generate the default assets in the `cluster` directory and launch a
full multi-node self-hosted setup with:

- 1 controller nodes
- 2 worker nodes

```
$ ./bootkube-up
```

## Cleaning up

To stop the running cluster and remove generated assets, run:

```
corectl halt $(corectl q | grep -E "^(etcd|worker|api-server)-" | tr "\n" " " )
rm -rf cluster *.qcow2
```
