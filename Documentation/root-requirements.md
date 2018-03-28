# Information about services running as root

Running services as a non-root user is typically better for security, since
permissions can be controlled.

Some Kubenetes services can be run as a non-root user, but most require root
privileges in some manner. This document will try out outline why certain
Kubernetes services require root privileges.

## Services that do *not* run as root

* Scheduler
* Controller Manager

## Services requiring root

### Flannel

Flannel is a layer 3 network fabric. Flannel needs access to low level
networking interfaces to be able to dynamically configure them.

### Proxy

kube-proxy is a network proxy found on every Kubernetes node. Kube-proxy often
needs to open and manage privileged ports (< 1024) in addition to managing iptables.

### DNS

DNS needs to bind to privileged port 53 for UDP and TCP.

### Checkpointer

Checkpointer is a service to recover a cluster after a reboot or loss of a node.
This service writes to `/etc/kubernetes/manifests`.
