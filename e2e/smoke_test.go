package e2e

import (
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

func TestSmoke(t *testing.T) {
	// 1. create nginx deployment
	di, _, err := api.Codecs.UniversalDecoder().Decode(nginxDep, nil, &v1beta1.Deployment{})
	if err != nil {
		t.Fatalf("unable to decode deployment manifest: %v", err)
	}
	d, ok := di.(*v1beta1.Deployment)
	if !ok {
		t.Fatalf("expected manifest to decode into *api.deployment, got %T", di)
	}
	_, err = client.ExtensionsV1beta1().Deployments(namespace).Create(d)
	if err != nil {
		t.Fatal(err)
	}
	defer client.ExtensionsV1beta1().Deployments(namespace).Delete("nginx-deployment", &metav1.DeleteOptions{})

	// 2. Get the nginx pod IP.
	var p *v1.Pod
	getPod := func() error {
		l, err := client.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: "app=smoke-nginx"})
		if err != nil || len(l.Items) == 0 {
			return fmt.Errorf("failed to list smoke-nginx pod: %v", err)
		}

		// take the first pod
		p = &l.Items[0]

		if p.Status.Phase != v1.PodRunning {
			return fmt.Errorf("smoke-nginx pod not running. Phase: %q, Reason %q: %v", p.Status.Phase, p.Status.Reason, p.Status.Message)
		}
		return nil
	}
	if err := retry(30, time.Second*10, getPod); err != nil {
		t.Fatalf("timed out waiting for nginx pod: %v", err)
	}

	// 3. Create a wget pod that hits the nginx pod IP.
	wgetPod.Spec.Containers[0].Command = []string{"wget", p.Status.PodIP}

	_, err = client.CoreV1().Pods(namespace).Create(wgetPod)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CoreV1().Pods(namespace).Delete(wgetPod.ObjectMeta.Name, &metav1.DeleteOptions{})

	getPod = func() error {
		p, err = client.CoreV1().Pods(namespace).Get("wget-pod", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to retrieve wget-pod: %v", err)
		}
		if p.Status.Phase != v1.PodSucceeded {
			return fmt.Errorf("wget-pod not running. Phase: %q, Reason %q: %v", p.Status.Phase, p.Status.Reason, p.Status.Message)
		}
		return nil
	}
	if err := retry(20, time.Second*10, getPod); err != nil {
		t.Fatalf(fmt.Sprintf("timed out waiting for wget pod to succeed: %v", err))
	}
}

var nginxDep = []byte(`apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: smoke-nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.8
        ports:
        - containerPort: 80`)

var wgetPod = &v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "wget-pod",
		Namespace: namespace,
	},
	Spec: v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "wget-container",
				Image: "busybox:1.26",
			},
		},
		RestartPolicy: v1.RestartPolicyNever,
	},
}
