# Preparing a bootkube release

### Update kubernetes vendor code

- Bump `VENDOR_VERSION` in `Makefile`
- Run `make vendor`

### Update hyperkube image

- Update hyperkube image version in templates: `pkg/asset/internal/templates.go`
- Update on-host kubelet versions (`KUBELET_VERSION`)
    - hack/multi-node/user-data.sample
    - hack/single-node/user-data.sample

### Update conformance test k8s version

- hack/tests/conformance-test.sh (`CONFORMANCE_VERSION`)

### Run conformance test

Easiest is to use internal jenkins jobs

Or, manually:

```
# GCE
./hack/tests/conformance-gce.sh
```

```
# Vagrant
make conformance-multi
```

```
# Other
./hack/tests/conformance-test.sh
```

### Cut a release image

Easiest is to use internal jenkins jobs

Or, manually:

```
PUSH_IMAGE=true ./build/build-image.sh
```

# Updating quickstart guides

Note: the quickstart guides use the release images, so we should not update them until after building/pushing new release.

Update on-host kubelet version (`KUBELET_VERSION`)

- hack/quickstart/kubelet.master
- hack/quickstart/kubelet.worker

Update the bootkube image version (to latest release)

- hack/quickstart/init-master.sh (`BOOTKUBE_VERSION`)

# Updating checkpointer

This only needs to happen when changes have been made to the checkpointer code / container.

### Build a new checkpointer image

Easiest is to use internal jenkin jobs

Or, manually:

```
BUILD_IMAGE=checkpoint PUSH_IMAGE=true ./build/build-image.sh
```

### Update checkpointer manifest

In `pkg/asset/internal/templates.go` change:

`CheckpointerTemplate` manifest to use the image built in previous step.

