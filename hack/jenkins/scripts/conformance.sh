#!/bin/bash
set -euo pipefail

export KUBECONFIG="${KUBECONFIG:-"${DIR}/../../quickstart/cluster/auth/kubeconfig"}"
export KUBERNETES_VERSION="v1.11.0"

# Set up kubectl
curl -L -O -v https://storage.googleapis.com/kubernetes-release/release/$KUBERNETES_VERSION/bin/linux/amd64/kubectl
chmod +x ./kubectl
export PATH=$PATH:$PWD
echo "kubectl setup completed"

# sonobuoy
# Conformance yaml file from https://raw.githubusercontent.com/cncf/k8s-conformance/master/sonobuoy-conformance.yaml
cat << EOF >> conformance.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: sonobuoy
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    component: sonobuoy
  name: sonobuoy-serviceaccount
  namespace: sonobuoy
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  labels:
    component: sonobuoy
  name: sonobuoy-serviceaccount
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: sonobuoy-serviceaccount
subjects:
- kind: ServiceAccount
  name: sonobuoy-serviceaccount
  namespace: sonobuoy
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  labels:
    component: sonobuoy
  name: sonobuoy-serviceaccount
  namespace: sonobuoy
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    component: sonobuoy
  name: sonobuoy-config-cm
  namespace: sonobuoy
data:
  config.json: |
    {
        "Description": "CNCF v1.9 or v.1.10 Conformance Results",
        "Filters": {
            "LabelSelector": "",
            "Namespaces": ".*"
        },
        "PluginNamespace": "sonobuoy",
        "Plugins": [
            {
                "name": "e2e"
            }
        ],
        "Resources": [
        ],
        "ResultsDir": "/tmp/sonobuoy",
        "Server": {
            "advertiseaddress": "sonobuoy-master:8080",
            "bindaddress": "0.0.0.0",
            "bindport": 8080,
            "timeoutseconds": 5400
        },
        "Version": "v0.10.0"
    }
---
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    component: sonobuoy
  name: sonobuoy-plugins-cm
  namespace: sonobuoy
data:
  e2e.tmpl: |
    apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        sonobuoy-driver: Job
        sonobuoy-plugin: e2e
        sonobuoy-result-type: e2e
      labels:
        component: sonobuoy
        sonobuoy-run: '{{.SessionID}}'
        tier: analysis
      name: sonobuoy-e2e-job-{{.SessionID}}
      namespace: '{{.Namespace}}'
    spec:
      containers:
      - env:
        - name: E2E_FOCUS
          value: '\[Conformance\]'
        image: gcr.io/heptio-images/kube-conformance:v1.10
        imagePullPolicy: Always
        name: e2e
        volumeMounts:
        - mountPath: /tmp/results
          name: results
          readOnly: false
      - command:
        - sh
        - -c
        - /sonobuoy worker global -v 5 --logtostderr
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: RESULTS_DIR
          value: /tmp/results
        - name: MASTER_URL
          value: '{{.MasterAddress}}'
        - name: RESULT_TYPE
          value: e2e
        image: gcr.io/heptio-images/sonobuoy:v0.10.0
        imagePullPolicy: Always
        name: sonobuoy-worker
        volumeMounts:
        - mountPath: /tmp/results
          name: results
          readOnly: false
      restartPolicy: Never
      serviceAccountName: sonobuoy-serviceaccount
      tolerations:
      - effect: NoSchedule
        key: node-role.kubernetes.io/master
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      volumes:
      - emptyDir: {}
        name: results
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: sonobuoy
    run: sonobuoy-master
    tier: analysis
  name: sonobuoy
  namespace: sonobuoy
spec:
  containers:
  - command:
    - /bin/bash
    - -c
    - /sonobuoy master --no-exit=true -v 3 --logtostderr
    env:
    - name: SONOBUOY_ADVERTISE_IP
      valueFrom:
        fieldRef:
          fieldPath: status.podIP
    image: gcr.io/heptio-images/sonobuoy:v0.10.0
    imagePullPolicy: Always
    name: kube-sonobuoy
    volumeMounts:
    - mountPath: /etc/sonobuoy
      name: sonobuoy-config-volume
    - mountPath: /plugins.d
      name: sonobuoy-plugins-volume
    - mountPath: /tmp/sonobuoy
      name: output-volume
  restartPolicy: Never
  serviceAccountName: sonobuoy-serviceaccount
  volumes:
  - configMap:
      name: sonobuoy-config-cm
    name: sonobuoy-config-volume
  - configMap:
      name: sonobuoy-plugins-cm
    name: sonobuoy-plugins-volume
  - emptyDir: {}
    name: output-volume
---
apiVersion: v1
kind: Service
metadata:
  labels:
    component: sonobuoy
    run: sonobuoy-master
  name: sonobuoy-master
  namespace: sonobuoy
spec:
  ports:
  - port: 8080
    protocol: TCP
    targetPort: 8080
  selector:
    run: sonobuoy-master
  type: ClusterIP
EOF

echo "Waiting for cluster to be up and ready"
# Wait for cluster to be ready
readyNodes() {
  kubectl --kubeconfig="${KUBECONFIG}" get nodes -o template --template='{{range .items}}{{range .status.conditions}}{{if eq .type "Ready"}}{{.}}{{end}}{{end}}{{end}}' | grep -o -E True | wc -l
}

until [[ "$(readyNodes)" == "2" ]]; do
  echo "$(readyNodes) of 2 nodes are Ready..."
  sleep 10
done

echo "Applying conformance tests"
kubectl --kubeconfig="${KUBECONFIG}" apply -f conformance.yaml

until [[ "$(kubectl --kubeconfig="${KUBECONFIG}" logs sonobuoy -n sonobuoy --tail=1)" == *"no-exit was specified, sonobuoy is now blocking"* ]]; do
  echo "Waiting for sonobuoy results"
  kubectl --kubeconfig="${KUBECONFIG}" logs sonobuoy -n sonobuoy --tail=1 || true
  sleep 10
done

# Copy results from pod to Jenkins executor
kubectl --kubeconfig="${KUBECONFIG}" cp sonobuoy/sonobuoy:/tmp/sonobuoy ${ARTIFACT_DIR}/conformance-results
