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
apiVersion: rbac.authorization.k8s.io/v1alpha1
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
    component: kubelet
spec:
  template:
    metadata:
      labels:
        tier: node
        component: kubelet
    spec:
      containers:
      - name: kubelet
        image: quay.io/coreos/hyperkube:v1.6.1_coreos.0
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
    component: kube-apiserver
spec:
  template:
    metadata:
      labels:
        tier: control-plane
        component: kube-apiserver
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      nodeSelector:
        master: "true"
      hostNetwork: true
      containers:
      - name: kube-apiserver
        image: quay.io/coreos/hyperkube:v1.6.1_coreos.0
        command:
        - /usr/bin/flock
        - --exclusive
        - --timeout=30
        - /var/lock/api-server.lock
        - /hyperkube
        - apiserver
        - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,ResourceQuota
        - --advertise-address=$(POD_IP)
        - --allow-privileged=true
        - --anonymous-auth=false
        - --authorization-mode=RBAC
        - --bind-address=0.0.0.0
        - --client-ca-file=/etc/kubernetes/secrets/ca.crt
        - --cloud-provider={{ .CloudProvider  }}
        - --etcd-servers={{ range $i, $e := .EtcdServers }}{{ if $i }},{{end}}{{ $e }}{{end}}
        - --insecure-port=8080
        - --kubelet-client-certificate=/etc/kubernetes/secrets/apiserver.crt
        - --kubelet-client-key=/etc/kubernetes/secrets/apiserver.key
        - --runtime-config=api/all=true
        - --secure-port=443
        - --service-account-key-file=/etc/kubernetes/secrets/service-account.pub
        - --service-cluster-ip-range={{ .ServiceCIDR }}
        - --storage-backend=etcd3
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

	KencTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kenc
  namespace: kube-system
  labels:
    tier: control-plane
    component: kenc
spec:
  template:
    metadata:
      labels:
        tier: control-plane
        component: kenc
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      nodeSelector:
        master: "true"
      hostNetwork: true
      containers:
      - image: quay.io/coreos/kenc:48b6feceeee56c657ea9263f47b6ea091e8d3035
        name: kenc
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
    component: pod-checkpointer
spec:
  template:
    metadata:
      labels:
        tier: control-plane
        component: pod-checkpointer
      annotations:
        checkpointer.alpha.coreos.com/checkpoint: "true"
    spec:
      nodeSelector:
        master: "true"
      hostNetwork: true
      containers:
      - name: checkpoint
        image: quay.io/coreos/pod-checkpointer:8a6356146761b512cc18ab77c8a421676f641c88
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
      restartPolicy: Always
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
    component: kube-controller-manager
spec:
  replicas: 2
  template:
    metadata:
      labels:
        tier: control-plane
        component: kube-controller-manager
    spec:
      nodeSelector:
        master: "true"
      containers:
      - name: kube-controller-manager
        image: quay.io/coreos/hyperkube:v1.6.1_coreos.0
        command:
        - ./hyperkube
        - controller-manager
        - --allocate-node-cidrs=true
        - --cloud-provider={{ .CloudProvider  }}
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
      volumes:
      - name: secrets
        secret:
          secretName: kube-controller-manager
      - name: ssl-host
        hostPath:
          path: /usr/share/ca-certificates
      dnsPolicy: Default # Don't use cluster DNS.
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
      component: kube-controller-manager
`)
	SchedulerTemplate = []byte(`apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: kube-scheduler
  namespace: kube-system
  labels:
    tier: control-plane
    component: kube-scheduler
spec:
  replicas: 2
  template:
    metadata:
      labels:
        tier: control-plane
        component: kube-scheduler
    spec:
      nodeSelector:
        master: "true"
      containers:
      - name: kube-scheduler
        image: quay.io/coreos/hyperkube:v1.6.1_coreos.0
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
      component: kube-scheduler
`)
	ProxyTemplate = []byte(`apiVersion: "extensions/v1beta1"
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
  labels:
    tier: node
    component: kube-proxy
spec:
  template:
    metadata:
      labels:
        tier: node
        component: kube-proxy
      annotations:
        scheduler.alpha.kubernetes.io/critical-pod: ''
    spec:
      hostNetwork: true
      containers:
      - name: kube-proxy
        image: quay.io/coreos/hyperkube:v1.6.1_coreos.0
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
      tolerations:
      - key: "CriticalAddonsOnly"
        operator: "Exists"
      volumes:
      - name: kube-dns-config
        configMap:
          name: kube-dns
          optional: true
      containers:
      - name: kubedns
        image: gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.1
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
        image: gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1
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
        image: gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.1
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
        image: quay.io/coreos/etcd-operator:v0.2.4
        env:
        - name: MY_POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: MY_POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
`)

	EtcdSvcTemplate = []byte(`apiVersion: v1
kind: Service
metadata:
  name: etcd-service
  namespace: kube-system
spec:
  selector:
    app: etcd
    etcd_cluster: kube-etcd
  clusterIP: {{ .ETCDServiceIP }}
  ports:
  - name: client
    port: 2379
    protocol: TCP
`)

	KubeFlannelCfgTemplate = []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-flannel-cfg
  namespace: kube-system
  labels:
    tier: node
    app: flannel
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
    app: flannel
spec:
  template:
    metadata:
      labels:
        tier: node
        app: flannel
    spec:
      hostNetwork: true
      containers:
      - name: kube-flannel
        image: quay.io/coreos/flannel:v0.7.0-amd64
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
        image: busybox
        command: [ "/bin/sh", "-c", "set -e -x; TMP=/etc/cni/net.d/.tmp-flannel-cfg; cp /etc/kube-flannel/cni-conf.json ${TMP}; mv ${TMP} /etc/cni/net.d/10-flannel.conf; while :; do sleep 3600; done" ]
        volumeMounts:
        - name: cni
          mountPath: /etc/cni/net.d
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
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
