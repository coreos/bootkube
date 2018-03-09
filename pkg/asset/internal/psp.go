package internal

var PSPPermissive = []byte(`apiVersion: extensions/v1beta1
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
