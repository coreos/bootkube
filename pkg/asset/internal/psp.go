package internal

var PSPPermissive = []byte(`apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: permissive
spec:
  privileged: true
  hostNetwork: true
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: RunAsAny
  fsGroup:
    rule: RunAsAny
  volumes:
  - 'configMap'
  - 'hostPath'
  - 'secret'
`)

var PSPPermissiveClusterRole = []byte(`kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: psp-permissive
rules:
- apiGroups: ["extensions"]
  resources: ["podsecuritypolicies"]
  resourceNames: ["permissive"]
  verbs: ["use"]
`)

var PSPKubeSystemRoleBinding = []byte(`
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: psp-permissive
  namespace: kube-system
subjects:
- kind: ServiceAccount
  name: replicaset-controller
  namespace: kube-system
- kind: ServiceAccount
  name: replication-controller
  namespace: kube-system
- kind: ServiceAccount
  name: job-controller
  namespace: kube-system
- kind: ServiceAccount
  name: daemon-set-controller
  namespace: kube-system
- kind: ServiceAccount
  name: statefulset-controller
  namespace: kube-system
roleRef:
  kind: ClusterRole
  name: psp-permissive
  apiGroup: rbac.authorization.k8s.io
`)

var PSPCoreDNS = []byte(`apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: coredns
  namespace: kube-system
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: 'docker/default'
    seccomp.security.alpha.kubernetes.io/defaultProfileName:  'docker/default'
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
  - ALL
  allowedCapabilities:
  - NET_BIND_SERVICE
  # Allow core volume types.
  volumes:
  - 'configMap'
  - 'secret'
  hostNetwork: false
  hostIPC: false
  hostPID: false
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'MustRunAs'
    ranges:
    - min: 1
      max: 65535
  fsGroup:
    rule: 'MustRunAs'
    ranges:
    - min: 1
      max: 65535
  readOnlyRootFilesystem: true
`)

var PSPCoreDNSRole = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: coredns-psp
  namespace: kube-system
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - coredns
`)

var PSPCoreDNSRoleBinding = []byte(`kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1beta1
metadata:
  name: coredns-psp
  namespace: kube-system
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: kube-system
roleRef:
  kind: Role
  name: coredns-psp
  apiGroup: rbac.authorization.k8s.io
`)
