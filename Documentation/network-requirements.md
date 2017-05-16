# Requirements

## Ports

The information below describes a minimum set of port allocations used by Kubernetes components.

### Master node(s) ingress

| Protocol | Port Range | Source                                    | Purpose                |
-----------|------------|-------------------------------------------|------------------------|
| TCP      | 443        | Worker Nodes, API Requests, and End-Users | Kubernetes API server. |
| UDP      | 8472       | Master & Worker Nodes                     | flannel overlay network - *vxlan backend* |

### etcd node(s) ingress

| Protocol | Port Range | Source                | Purpose                                          |
-----------|------------|-----------------------|--------------------------------------------------|
| TCP      | 2379-2380  | Master & Worker Nodes | etcd server client API                           |

### Worker node(s) ingress

| Protocol | Port Range  | Source                         | Purpose                                                                |
-----------|-------------|--------------------------------|------------------------------------------------------------------------|
| TCP      | 4194        | Master & Worker Nodes          | The port of the localhost cAdvisor endpoint |
| UDP      | 8472        | Master & Worker Nodes          | flannel overlay network - *vxlan backend* |
| TCP      | 10250       | Master Nodes                   | Worker node Kubelet API for exec and logs.                                  |
| TCP      | 10255       | Master & Worker Nodes          | Worker node read-only Kubelet API (Heapster).                                  |
| TCP      | 30000-32767 | External Application Consumers | Default port range for [external service][https://kubernetes.io/docs/concepts/services-networking/service] ports. Typically, these ports would need to be exposed to external load-balancers, or other external consumers of the application itself. |
