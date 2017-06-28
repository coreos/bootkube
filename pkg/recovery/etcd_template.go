package recovery

var recoveryEtcdTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: recovery-etcd
  namespace: kube-system
  labels:
    k8s-app: recovery-etcd
spec:
  initContainers:
  - name: recovery
    image: {{ .Image }}
    command: 
    - /usr/local/bin/etcdctl
    - snapshot
    - restore
    - --data-dir=/var/etcd/recovery
    - --name=recovery-etcd
    - --initial-cluster=recovery-etcd=http://localhost:52380
    - --initial-cluster-token=bootkube-recovery
    - --initial-advertise-peer-urls=http://localhost:52380
    - --skip-hash-check=true
    - /var/etcd-backupdir/{{ .BackupFile }}
    env:
    - name: ETCDCTL_API
      value: "3"
    volumeMounts:
      - mountPath: /var/etcd
        name: etcd
        readOnly: false
      - mountPath: /var/etcd-backupdir
        name: etcdbackup
        readOnly: false
  containers:
  - name: etcd
    image: {{ .Image }}
    command:
    - /usr/local/bin/etcd
    - --name=recovery-etcd
    - --listen-client-urls={{ .ClientAddr }}
    - --listen-peer-urls=http://0.0.0.0:52380
    - --advertise-client-urls={{ .ClientAddr }}
    - --data-dir=/var/etcd/recovery
    volumeMounts:
      - mountPath: /var/etcd
        name: etcd
        readOnly: false
  hostNetwork: true
  restartPolicy: Never
  volumes:
    - name: etcd
      emptyDir: {}
    - name: etcdbackup
      hostPath:
        path: {{ .BackupDir }}
`)

var bootFromBackupEtcdTemplate = []byte(`apiVersion: v1
kind: Pod
metadata:
  name: bootstrap-etcd
  namespace: kube-system
  labels:
    k8s-app: boot-etcd
spec:
  initContainers:
  - name: recovery
    image: {{ .Image }}
    command:
    - /bin/sh 
    - -ec
    - |
      etcdctl snapshot restore \
      /var/etcd-backupdir/{{ .BackupFile }} \
      --data-dir=/var/etcd/data \
      --name=boot-etcd \
      --initial-cluster=boot-etcd=https://{{ .BootEtcdServiceIP }}:12380 \
      --initial-cluster-token={{ .ClusterToken }} \
      --initial-advertise-peer-urls=https://{{ .BootEtcdServiceIP }}:12380 \
      --skip-hash-check=true 
    env:
      - name: ETCDCTL_API
        value: "3"
    volumeMounts:
        - mountPath: /var/etcd
          name: etcd
          readOnly: false
        - mountPath: /var/etcd-backupdir
          name: etcdbackup
          readOnly: false
  - name: cleanup
    image: {{ .Image }}
    command:
    - /bin/sh 
    - -ec
    - |
      (/usr/local/bin/etcd \
      --listen-client-urls=http://0.0.0.0:32379 \
      --listen-peer-urls=http://0.0.0.0:32380 \
      --advertise-client-urls=http://localhost:32379 \
      --data-dir=/var/etcd/data &) && sleep 30 && \
      etcdctl \
      --endpoints=http://localhost:32379 \
      del {{ .TPRKey }} && \
      etcdctl \
      --endpoints=http://localhost:32379 \
      del --prefix {{ .MemberPodPrefix }}
    env:
      - name: ETCDCTL_API
        value: "3"
    volumeMounts:
        - mountPath: /var/etcd
          name: etcd
          readOnly: false
  containers:
  - name: etcd
    image: {{ .Image }}
    command:
    - /usr/local/bin/etcd
    - --name=boot-etcd
    - --listen-client-urls=https://0.0.0.0:12379
    - --listen-peer-urls=https://0.0.0.0:12380
    - --advertise-client-urls=https://{{ .BootEtcdServiceIP }}:12379
    - --data-dir=/var/etcd/data
    - --peer-client-cert-auth=true
    - --peer-trusted-ca-file=/etc/kubernetes/secrets/peer-ca-crt.pem
    - --peer-cert-file=/etc/kubernetes/secrets/peer-crt.pem
    - --peer-key-file=/etc/kubernetes/secrets/peer-key.pem
    - --client-cert-auth=true
    - --trusted-ca-file=/etc/kubernetes/secrets/client-ca-crt.pem
    - --cert-file=/etc/kubernetes/secrets/client-crt.pem
    - --key-file=/etc/kubernetes/secrets/client-key.pem
    volumeMounts:
      - mountPath: /var/etcd
        name: etcd
        readOnly: false
      - mountPath: /etc/kubernetes/secrets
        name: secrets
        readOnly: true
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  restartPolicy: Never
  volumes:
    - name: etcd
      emptyDir: {}
    - name: etcdbackup
      hostPath:
        path: {{ .BackupDir }}
    - name: secrets
      hostPath:
        path: /etc/kubernetes/bootstrap-secrets
`)

var recoveryEtcdSvcTemplate = []byte(`{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "name": "bootstrap-etcd-service",
    "namespace": "kube-system"
  },
  "spec": {
    "selector": {
      "k8s-app": "boot-etcd"
    },
    "clusterIP": "{{ .BootEtcdServiceIP }}",
    "ports": [
      {
        "name": "client",
        "port": 12379,
        "protocol": "TCP"
      },
      {
        "name": "peers",
        "port": 12380,
        "protocol": "TCP"
      }
    ]
  }
}`)
