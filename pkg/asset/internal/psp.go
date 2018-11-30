package internal

var PSPPermissive = []byte(`
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: permissive
spec:
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  privileged: true
  hostNetwork: true
  hostPorts:
  - min: 0
    max: 65535
  hostIPC: true
  hostPID: true
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  runAsUser:
    rule: RunAsAny
  fsGroup:
    rule: RunAsAny
  readOnlyRootFilesystem: false
  volumes:
  - '*'
`)

var PSPPermissiveClusterRole = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: psp-permissive
rules:
- apiGroups: ["policy"]
  resources: ["podsecuritypolicies"]
  resourceNames: ["permissive"]
  verbs: ["use"]
`)

var PSPKubeSystemRoleBinding = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: psp-permissive
  namespace: kube-system
subjects:
- kind: ServiceAccount
  name: pod-checkpointer 
  namespace: kube-system
- kind: ServiceAccount
  name: kube-controller-manager
  namespace: kube-system
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:nodes
- kind: User
  apiGroup: rbac.authorization.k8s.io
  # Legacy node ID
  name: kubelet
roleRef:
  kind: ClusterRole
  name: psp-permissive
  apiGroup: rbac.authorization.k8s.io
`)

var PSPDefaultPermissive = []byte(`
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: permissive-default
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: 'docker/default'
    seccomp.security.alpha.kubernetes.io/defaultProfileName:  'docker/default'
spec:
  privileged: false
  allowPrivilegeEscalation: true
  volumes:
  - 'configMap'
  - 'emptyDir'
  - 'projected'
  - 'secret'
  - 'downwardAPI'
  - 'persistentVolumeClaim'
  hostNetwork: false
  hostIPC: false
  hostPID: false
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: MustRunAs
    ranges:
    - min: 1
      max: 65535
  fsGroup:
    rule: 'MustRunAs'
    ranges:
    - min: 1
      max: 65535
  readOnlyRootFilesystem: false
`)

var PSPDefaultPermissiveClusterRole = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: psp-permissive-default
  namespace: default
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  resourceNames: ['permissive-default']
  verbs: ['use']
`)

var PSPDefaultPermissiveClusterRoleBinding = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: psp-permissive-default
  namespace: default
subjects:
- kind: Group
  name: system:serviceaccounts:default
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: psp-permissive-default
  apiGroup: rbac.authorization.k8s.io
`)

var PSPCoreDNS = []byte(`
apiVersion: policy/v1beta1
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

var PSPCoreDNSRole = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: coredns-psp
  namespace: kube-system
rules:
- apiGroups:     ['policy']
  resources:     ['podsecuritypolicies']
  resourceNames: ['coredns']
  verbs:         ['use']
`)

var PSPCoreDNSRoleBinding = []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
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
