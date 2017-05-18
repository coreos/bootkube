// Package internal holds asset templates used by bootkube.
package internal

var (
	KubeConfigTemplate = []byte(`apiVersion: v1
kind: Config
clusters:
- name: local
  cluster:
    server: {{ .Server }}
    certificate-authority-data: {{ .CACert }}
users:
- name: kubelet
  user:
    client-certificate-data: {{ .KubeletCert}}
    client-key-data: {{ .KubeletKey }}
contexts:
- context:
    cluster: local
    user: kubelet
`)

	KubeSystemSARoleBindingTemplate = []byte(`apiVersion: rbac.authorization.k8s.io/v1alpha1
kind: ClusterRoleBinding
metadata:
  name: system:default-sa
subjects:
  - kind: ServiceAccount
    name: default
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
`)

	KubeletTemplate = []byte(`apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kubelet
  namespace: kube-system
  labels:
    tier: node
    k8s-app: kubelet
spec:
  template:
    metadata:
      labels:
        tier: node
        k8s-app: kubelet
    spec:
      containers:
      - name: kubelet
        image: {{ .Images.Hyperkube }}
        command:
        - ./hyperkube
        - kubelet
        - --allow-privileged
        - --cluster-dns={{ .DNSServiceIP }}
        - --cluster-domain=cluster.local
        - --cni-conf-dir=/etc/kubernetes/cni/net.d
        - --cni-bin-dir=/opt/cni/bin
        - --containerized
        - --hostname-override=$(NODE_NAME)
        - --kubeconfig=/etc/kubernetes/kubeconfig
        - --lock-file=/var/run/lock/kubelet.lock
        - --network-plugin=cni
        - --pod-manifest-path=/etc/kubernetes/manifests
        - --require-kubeconfig
        env:
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
        securityContext:
          privileged: true
        volumeMounts:
        - name: dev
          mountPath: /dev
        - name: run
          mountPath: /run
        - name: sys
          mountPath: /sys
          readOnly: true
        - name: etc-kubernetes
          mountPath: /etc/kubernetes
          readOnly: true
        - name: etc-ssl-certs
          mountPath: /etc/ssl/certs
          readOnly: true
        - name: var-lib-docker
          mountPath: /var/lib/docker
        - name: var-lib-kubelet
          mountPath: /var/lib/kubelet
        - name: var-lib-rkt
          mountPath: /var/lib/rkt
        - name: rootfs
          mountPath: /rootfs
      hostNetwork: true
      hostPID: true
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: dev
        hostPath:
          path: /dev
      - name: run
        hostPath:
          path: /run
      - name: sys
        hostPath:
          path: /sys
      - name: etc-kubernetes
        hostPath:
          path: /etc/kubernetes
      - name: etc-ssl-certs
        hostPath:
          path: /usr/share/ca-certificates
      - name: var-lib-docker
        hostPath:
          path: /var/lib/docker
      - name: var-lib-kubelet
        hostPath:
          path: /var/lib/kubelet
      - name: var-lib-rkt
        hostPath:
          path: /var/lib/rkt
      - name: rootfs
        hostPath:
          path: /
`)

	APIServerTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kube-apiserver
  namespace: kube-system
  labels:
    tier: control-plane
    k8s-app: kube-apiserver
spec:
  template:
    metadata:
      labels:
        tier: control-plane
        k8s-app: kube-apiserver
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      containers:
      - name: kube-apiserver
        image: {{ .Images.Hyperkube }}
        command:
        - /usr/bin/flock
        - --exclusive
        - --timeout=30
        - /var/lock/api-server.lock
        - /hyperkube
        - apiserver
        - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota
        - --advertise-address=$(POD_IP)
        - --allow-privileged=true
        - --anonymous-auth=false
        - --authorization-mode=RBAC
        - --bind-address=0.0.0.0
        - --client-ca-file=/etc/kubernetes/secrets/ca.crt
        - --cloud-provider={{ .CloudProvider }}
{{- if .EtcdUseTLS }}
        - --etcd-cafile=/etc/kubernetes/secrets/etcd-ca.crt
        - --etcd-certfile=/etc/kubernetes/secrets/etcd-client.crt
        - --etcd-keyfile=/etc/kubernetes/secrets/etcd-client.key
{{- end }}
        - --etcd-servers={{ range $i, $e := .EtcdServers }}{{ if $i }},{{end}}{{ $e }}{{end}}
        - --insecure-port=0
        - --kubelet-client-certificate=/etc/kubernetes/secrets/apiserver.crt
        - --kubelet-client-key=/etc/kubernetes/secrets/apiserver.key
        - --secure-port=443
        - --service-account-key-file=/etc/kubernetes/secrets/service-account.pub
        - --service-cluster-ip-range={{ .ServiceCIDR }}
        - --storage-backend=etcd3
        - --tls-ca-file=/etc/kubernetes/secrets/ca.crt
        - --tls-cert-file=/etc/kubernetes/secrets/apiserver.crt
        - --tls-private-key-file=/etc/kubernetes/secrets/apiserver.key
        env:
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        volumeMounts:
        - mountPath: /etc/ssl/certs
          name: ssl-certs-host
          readOnly: true
        - mountPath: /etc/kubernetes/secrets
          name: secrets
          readOnly: true
        - mountPath: /var/lock
          name: var-lock
          readOnly: false
      hostNetwork: true
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: ssl-certs-host
        hostPath:
          path: /usr/share/ca-certificates
      - name: secrets
        secret:
          secretName: kube-apiserver
      - name: var-lock
        hostPath:
          path: /var/lock
`)

	BootstrapAPIServerTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: bootstrap-kube-apiserver
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    image: {{ .Images.Hyperkube }}
    command:
    - /usr/bin/flock
    - --exclusive
    - --timeout=30
    - /var/lock/api-server.lock
    - /hyperkube
    - apiserver
    - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota
    - --advertise-address=$(POD_IP)
    - --allow-privileged=true
    - --authorization-mode=RBAC
    - --bind-address=0.0.0.0
    - --client-ca-file=/etc/kubernetes/secrets/ca.crt
{{- if .EtcdUseTLS }}
    - --etcd-cafile=/etc/kubernetes/secrets/etcd-ca.crt
    - --etcd-certfile=/etc/kubernetes/secrets/etcd-client.crt
    - --etcd-keyfile=/etc/kubernetes/secrets/etcd-client.key
{{- end }}
    - --etcd-servers={{ range $i, $e := .EtcdServers }}{{ if $i }},{{end}}{{ $e }}{{end}}{{ if .SelfHostedEtcd }},http://127.0.0.1:4001{{end}}
    - --insecure-port=0
    - --kubelet-client-certificate=/etc/kubernetes/secrets/apiserver.crt
    - --kubelet-client-key=/etc/kubernetes/secrets/apiserver.key
    - --secure-port=443
    - --service-account-key-file=/etc/kubernetes/secrets/service-account.pub
    - --service-cluster-ip-range={{ .ServiceCIDR }}
    - --cloud-provider={{ .CloudProvider }}
    - --storage-backend=etcd3
    - --tls-ca-file=/etc/kubernetes/secrets/ca.crt
    - --tls-cert-file=/etc/kubernetes/secrets/apiserver.crt
    - --tls-private-key-file=/etc/kubernetes/secrets/apiserver.key
    env:
    - name: POD_IP
      valueFrom:
        fieldRef:
          fieldPath: status.podIP
    volumeMounts:
    - mountPath: /etc/ssl/certs
      name: ssl-certs-host
      readOnly: true
    - mountPath: /etc/kubernetes/secrets
      name: secrets
      readOnly: true
    - mountPath: /var/lock
      name: var-lock
      readOnly: false
  hostNetwork: true
  volumes:
  - name: secrets
    hostPath:
      path: /etc/kubernetes/{{ .BootstrapSecretsSubdir }}
  - name: ssl-certs-host
    hostPath:
      path: /usr/share/ca-certificates
  - name: var-lock
    hostPath:
      path: /var/lock
`)

	KencTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kube-etcd-network-checkpointer
  namespace: kube-system
  labels:
    tier: control-plane
    k8s-app: kube-etcd-network-checkpointer
spec:
  template:
    metadata:
      labels:
        tier: control-plane
        k8s-app: kube-etcd-network-checkpointer
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      containers:
      - image: {{ .Images.Kenc }}
        name: kube-etcd-network-checkpointer
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /etc/kubernetes/selfhosted-etcd
          name: checkpoint-dir
          readOnly: false
        - mountPath: /var/lock
          name: var-lock
          readOnly: false
        command:
        - /usr/bin/flock
        - /var/lock/kenc.lock
        - -c
        - "kenc -r -m iptables && kenc -m iptables"
      hostNetwork: true
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: checkpoint-dir
        hostPath:
          path: /etc/kubernetes/checkpoint-iptables
      - name: var-lock
        hostPath:
          path: /var/lock
`)

	CheckpointerTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: pod-checkpointer
  namespace: kube-system
  labels:
    tier: control-plane
    k8s-app: pod-checkpointer
spec:
  template:
    metadata:
      labels:
        tier: control-plane
        k8s-app: pod-checkpointer
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      containers:
      - name: checkpoint
        image: {{ .Images.PodCheckpointer }}
        command:
        - /checkpoint
        - --v=4
        - --lock-file=/var/run/lock/pod-checkpointer.lock
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        imagePullPolicy: Always
        volumeMounts:
        - mountPath: /etc/kubernetes
          name: etc-kubernetes
        - mountPath: /var/run
          name: var-run
      hostNetwork: true
      nodeSelector:
        node-role.kubernetes.io/master: ""
      restartPolicy: Always
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: etc-kubernetes
        hostPath:
          path: /etc/kubernetes
      - name: var-run
        hostPath:
          path: /var/run
`)
	ControllerManagerTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-controller-manager
  namespace: kube-system
  labels:
    tier: control-plane
    k8s-app: kube-controller-manager
spec:
  replicas: 2
  template:
    metadata:
      labels:
        tier: control-plane
        k8s-app: kube-controller-manager
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: tier
                  operator: In
                  values:
                  - control-plane
                - key: k8s-app
                  operator: In
                  values:
                  - kube-contoller-manager
              topologyKey: kubernetes.io/hostname
      containers:
      - name: kube-controller-manager
        image: {{ .Images.Hyperkube }}
        command:
        - ./hyperkube
        - controller-manager
        - --allocate-node-cidrs=true
        - --cloud-provider={{ .CloudProvider }}
        - --cluster-cidr={{ .PodCIDR }}
        - --configure-cloud-routes=false
        - --leader-elect=true
        - --root-ca-file=/etc/kubernetes/secrets/ca.crt
        - --service-account-private-key-file=/etc/kubernetes/secrets/service-account.key
        livenessProbe:
          httpGet:
            path: /healthz
            port: 10252  # Note: Using default port. Update if --port option is set differently.
          initialDelaySeconds: 15
          timeoutSeconds: 15
        volumeMounts:
        - name: secrets
          mountPath: /etc/kubernetes/secrets
          readOnly: true
        - name: ssl-host
          mountPath: /etc/ssl/certs
          readOnly: true
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: secrets
        secret:
          secretName: kube-controller-manager
      - name: ssl-host
        hostPath:
          path: /usr/share/ca-certificates
      dnsPolicy: Default # Don't use cluster DNS.
`)

	BootstrapControllerManagerTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: bootstrap-kube-controller-manager
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    image: {{ .Images.Hyperkube }}
    command:
    - ./hyperkube
    - controller-manager
    - --allocate-node-cidrs=true
    - --cluster-cidr={{ .PodCIDR }}
    - --cloud-provider={{ .CloudProvider }}
    - --configure-cloud-routes=false
    - --kubeconfig=/etc/kubernetes/kubeconfig
    - --leader-elect=true
    - --root-ca-file=/etc/kubernetes/{{ .BootstrapSecretsSubdir }}/ca.crt
    - --service-account-private-key-file=/etc/kubernetes/{{ .BootstrapSecretsSubdir }}/service-account.key
    volumeMounts:
    - name: kubernetes
      mountPath: /etc/kubernetes
      readOnly: true
    - name: ssl-host
      mountPath: /etc/ssl/certs
      readOnly: true
  hostNetwork: true
  volumes:
  - name: kubernetes
    hostPath:
      path: /etc/kubernetes
  - name: ssl-host
    hostPath:
      path: /usr/share/ca-certificates
`)
	ControllerManagerDisruptionTemplate = []byte(`apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: kube-controller-manager
  namespace: kube-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      tier: control-plane
      k8s-app: kube-controller-manager
`)
	SchedulerTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-scheduler
  namespace: kube-system
  labels:
    tier: control-plane
    k8s-app: kube-scheduler
spec:
  replicas: 2
  template:
    metadata:
      labels:
        tier: control-plane
        k8s-app: kube-scheduler
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: tier
                  operator: In
                  values:
                  - control-plane
                - key: k8s-app
                  operator: In
                  values:
                  - kube-scheduler
              topologyKey: kubernetes.io/hostname
      containers:
      - name: kube-scheduler
        image: {{ .Images.Hyperkube }}
        command:
        - ./hyperkube
        - scheduler
        - --leader-elect=true
        livenessProbe:
          httpGet:
            path: /healthz
            port: 10251  # Note: Using default port. Update if --port option is set differently.
          initialDelaySeconds: 15
          timeoutSeconds: 15
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
`)

	BootstrapSchedulerTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: bootstrap-kube-scheduler
  namespace: kube-system
spec:
  containers:
  - name: kube-scheduler
    image: {{ .Images.Hyperkube }}
    command:
    - ./hyperkube
    - scheduler
    - --kubeconfig=/etc/kubernetes/kubeconfig
    - --leader-elect=true
    volumeMounts:
    - name: kubernetes
      mountPath: /etc/kubernetes
      readOnly: true
  hostNetwork: true
  volumes:
  - name: kubernetes
    hostPath:
      path: /etc/kubernetes
`)
	SchedulerDisruptionTemplate = []byte(`apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: kube-scheduler
  namespace: kube-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      tier: control-plane
      k8s-app: kube-scheduler
`)
	ProxyTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
  labels:
    tier: node
    k8s-app: kube-proxy
spec:
  template:
    metadata:
      labels:
        tier: node
        k8s-app: kube-proxy
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      containers:
      - name: kube-proxy
        image: {{ .Images.Hyperkube }}
        command:
        - /hyperkube
        - proxy
        - --cluster-cidr={{ .PodCIDR }}
        - --hostname-override=$(NODE_NAME)
        - --kubeconfig=/etc/kubernetes/kubeconfig
        - --proxy-mode=iptables
        env:
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /etc/ssl/certs
          name: ssl-certs-host
          readOnly: true
        - name: etc-kubernetes
          mountPath: /etc/kubernetes
          readOnly: true
      hostNetwork: true
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - hostPath:
          path: /usr/share/ca-certificates
        name: ssl-certs-host
      - name: etc-kubernetes
        hostPath:
          path: /etc/kubernetes
`)
	DNSDeploymentTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
spec:
  # replicas: not specified here:
  # 1. In order to make Addon Manager do not reconcile this replicas parameter.
  # 2. Default is 1.
  # 3. Will be tuned in real time if DNS horizontal auto-scaling is turned on.
  strategy:
    rollingUpdate:
      maxSurge: 10%
      maxUnavailable: 0
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      containers:
      - name: kubedns
        image: {{ .Images.KubeDNS }}
        resources:
          # TODO: Set memory limits when we've profiled the container for large
          # clusters, then set request = limit to keep this container in
          # guaranteed class. Currently, this container falls into the
          # "burstable" category so the kubelet doesn't backoff from restarting it.
          limits:
            memory: 170Mi
          requests:
            cpu: 100m
            memory: 70Mi
        livenessProbe:
          httpGet:
            path: /healthcheck/kubedns
            port: 10054
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        readinessProbe:
          httpGet:
            path: /readiness
            port: 8081
            scheme: HTTP
          # we poll on pod startup for the Kubernetes master service and
          # only setup the /readiness HTTP server once that's available.
          initialDelaySeconds: 3
          timeoutSeconds: 5
        args:
        - --domain=cluster.local.
        - --dns-port=10053
        - --config-dir=/kube-dns-config
        - --v=2
        env:
        - name: PROMETHEUS_PORT
          value: "10055"
        ports:
        - containerPort: 10053
          name: dns-local
          protocol: UDP
        - containerPort: 10053
          name: dns-tcp-local
          protocol: TCP
        - containerPort: 10055
          name: metrics
          protocol: TCP
        volumeMounts:
        - name: kube-dns-config
          mountPath: /kube-dns-config
      - name: dnsmasq
        image: {{ .Images.KubeDNSMasq }}
        livenessProbe:
          httpGet:
            path: /healthcheck/dnsmasq
            port: 10054
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        args:
        - -v=2
        - -logtostderr
        - -configDir=/etc/k8s/dns/dnsmasq-nanny
        - -restartDnsmasq=true
        - --
        - -k
        - --cache-size=1000
        - --log-facility=-
        - --server=/cluster.local/127.0.0.1#10053
        - --server=/in-addr.arpa/127.0.0.1#10053
        - --server=/ip6.arpa/127.0.0.1#10053
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        # see: https://github.com/kubernetes/kubernetes/issues/29055 for details
        resources:
          requests:
            cpu: 150m
            memory: 20Mi
        volumeMounts:
        - name: kube-dns-config
          mountPath: /etc/k8s/dns/dnsmasq-nanny
      - name: sidecar
        image: {{ .Images.KubeDNSSidecar }}
        livenessProbe:
          httpGet:
            path: /metrics
            port: 10054
            scheme: HTTP
          initialDelaySeconds: 60
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 5
        args:
        - --v=2
        - --logtostderr
        - --probe=kubedns,127.0.0.1:10053,kubernetes.default.svc.cluster.local,5,A
        - --probe=dnsmasq,127.0.0.1:53,kubernetes.default.svc.cluster.local,5,A
        ports:
        - containerPort: 10054
          name: metrics
          protocol: TCP
        resources:
          requests:
            memory: 20Mi
            cpu: 10m
      dnsPolicy: Default  # Don't use cluster DNS.
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - key: CriticalAddonsOnly
        operator: Exists
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
      - name: kube-dns-config
        configMap:
          name: kube-dns
          optional: true
`)
	DNSSvcTemplate = []byte(`apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "KubeDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: {{ .DNSServiceIP }}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
`)

	EtcdOperatorTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: etcd-operator
  namespace: kube-system
  labels:
    k8s-app: etcd-operator
spec:
  replicas: 1
  template:
    metadata:
      labels:
        k8s-app: etcd-operator
    spec:
      containers:
      - name: etcd-operator
        image: {{ .Images.EtcdOperator }}
        env:
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
      nodeSelector:
        node-role.kubernetes.io/master: ""
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
`)

	EtcdSvcTemplate = []byte(`apiVersion: v1
kind: Service
metadata:
  name: {{ .EtcdServiceName }}
  namespace: kube-system
spec:
  selector:
    app: etcd
    etcd_cluster: kube-etcd
  clusterIP: {{ .EtcdServiceIP }}
  ports:
  - name: client
    port: 2379
    protocol: TCP
`)

	BootstrapEtcdTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: bootstrap-etcd
  namespace: kube-system
  labels:
    k8s-app: boot-etcd
spec:
  containers:
  - name: etcd
    image: {{ .Images.Etcd }}
    command:
    - /usr/local/bin/etcd
    - --name=boot-etcd
    - --listen-client-urls=http://0.0.0.0:4001
    - --listen-peer-urls=http://0.0.0.0:12380
    - --advertise-client-urls=http://$(MY_POD_IP):4001
    - --initial-advertise-peer-urls=http://$(MY_POD_IP):12380
    - --initial-cluster=boot-etcd=http://$(MY_POD_IP):12380
    - --initial-cluster-token=bootkube
    - --initial-cluster-state=new
    - --data-dir=/var/etcd/data
    env:
      - name: MY_POD_IP
        valueFrom:
          fieldRef:
            fieldPath: status.podIP
  hostNetwork: true
  restartPolicy: Never
`)

	KubeFlannelCfgTemplate = []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-flannel-cfg
  namespace: kube-system
  labels:
    tier: node
    k8s-app: flannel
data:
  cni-conf.json: |
    {
      "name": "cbr0",
      "type": "flannel",
      "delegate": {
        "isDefaultGateway": true
      }
    }
  net-conf.json: |
    {
      "Network": "{{ .PodCIDR }}",
      "Backend": {
        "Type": "vxlan"
      }
    }
`)

	KubeFlannelTemplate = []byte(`apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: kube-flannel
  namespace: kube-system
  labels:
    tier: node
    k8s-app: flannel
spec:
  template:
    metadata:
      labels:
        tier: node
        k8s-app: flannel
    spec:
      containers:
      - name: kube-flannel
        image: {{ .Images.Flannel }}
        command: [ "/opt/bin/flanneld", "--ip-masq", "--kube-subnet-mgr", "--iface=$(POD_IP)"]
        securityContext:
          privileged: true
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        volumeMounts:
        - name: run
          mountPath: /run
        - name: cni
          mountPath: /etc/cni/net.d
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      - name: install-cni
        image: {{ .Images.Busybox }}
        command: [ "/bin/sh", "-c", "set -e -x; TMP=/etc/cni/net.d/.tmp-flannel-cfg; cp /etc/kube-flannel/cni-conf.json ${TMP}; mv ${TMP} /etc/cni/net.d/10-flannel.conf; while :; do sleep 3600; done" ]
        volumeMounts:
        - name: cni
          mountPath: /etc/cni/net.d
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      hostNetwork: true
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      volumes:
        - name: run
          hostPath:
            path: /run
        - name: cni
          hostPath:
            path: /etc/kubernetes/cni/net.d
        - name: flannel-cfg
          configMap:
            name: kube-flannel-cfg
`)
)
