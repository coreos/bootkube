# Bootkube Roadmap

## v1.0.0 targets

- [X] Self-hosted etcd + TLS (https://github.com/kubernetes-incubator/bootkube/releases/tag/v0.4.5)
- [ ] Self-hosted etcd is stable and enabled by default
- [ ] Recovery from etcd-backup part of e2e testing (https://github.com/kubernetes-incubator/bootkube/issues/596)
- [ ] Publicly published upstream conformance tests
- [ ] How-it-works documentation for bootkube + self-hosted etcd
- [ ] Documentation for running HA clusters (https://github.com/kubernetes-incubator/bootkube/issues/311)
- [ ] Versioned configuration objects replace flags (https://github.com/kubernetes-incubator/bootkube/issues/565)

## Upstream Features (as available)

- [ ] Kubelet TLS bootstrap/rotation for client/server certificates (https://github.com/kubernetes/features/issues/43 & https://github.com/kubernetes/features/issues/266)
- [ ] componentConfig/configMap for all core components
- [ ] Cluster configuration object (https://github.com/kubernetes/kubernetes/issues/19831)
- [ ] Adoption of kubelet checkpointing / deprecation of pod-checkpointer (https://github.com/kubernetes/kubernetes/issues/489)
