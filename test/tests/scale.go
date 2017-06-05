// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tests

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/mantle/pluton"
	"github.com/coreos/mantle/util"
)

func EtcdScale(c *pluton.Cluster) {
	// add two master nodes to cluster
	if err := c.AddMasters(2); err != nil {
		c.Fatal(err)
	}

	// scale up etcd operator
	if err := resizeSelfHostedEtcd(c, 3); err != nil {
		c.Fatalf("scaling up: %v", err)
	}

	// todo check that each pod runs on a different master node
	if err := checkEtcdPodDistribution(c, 3); err != nil {
		c.Fatal(err)
	}

	// scale back to 1
	if err := resizeSelfHostedEtcd(c, 1); err != nil {
		c.Fatalf("scaling down: %v", err)
	}

	// run an nginx deployment and ping it
	if err := nginxCheck(c); err != nil {
		c.Fatalf("nginxCheck: %v", err)
	}
}

// resizes self-hosted etcd and checks that the desired number of pods are in a running state
func resizeSelfHostedEtcd(c *pluton.Cluster, size int) error {
	const (
		tprGroup   = "etcd.coreos.com"
		apiVersion = "v1beta1"
		tprKind    = "clusters"
	)
	var tprEndpoint = fmt.Sprintf("http://127.0.0.1:8080/apis/%s/%s/namespaces/kube-system/%s/kube-etcd",
		tprGroup, apiVersion, tprKind)

	scaleCmds := []string{
		fmt.Sprintf("curl -f -H  'Content-Type: application/json' -X GET %v > body.json", tprEndpoint),
		// delete resourceVersion field before curling back
		fmt.Sprintf("jq 'recurse(.metadata) |= del(.resourceVersion)' < body.json | jq .spec.size=%v > newbody.json", size),
		fmt.Sprintf("curl -f -H  'Content-Type: application/json' -X PUT --data @newbody.json %v", tprEndpoint),
	}
	for _, cmd := range scaleCmds {
		sout, serr, err := c.SSH(cmd)
		if err != nil {
			return fmt.Errorf("error in scale up command: %v:\nSTDERR: %s\nSTDOUT: %s", cmd, serr, sout)
		}
	}

	// check that all pods are running
	podsReady := func() error {
		out, err := c.Kubectl(`get po -l etcd_cluster=kube-etcd -o jsonpath='{.items[*].status.phase}' --namespace=kube-system`)
		if err != nil {
			return err
		}
		phases := strings.Split(out, " ")
		if len(phases) != size {
			return fmt.Errorf("expected %d etcd pods got %d: %v", size, len(phases), phases)
		}
		for _, phase := range phases {
			if phase != "Running" {
				return fmt.Errorf("one or more etcd pods not in a 'Running' phase")
			}
		}
		return nil
	}

	if err := util.Retry(15, 10*time.Second, podsReady); err != nil {
		return fmt.Errorf("Waited 150 seconds for etcd to scale: %v", err)
	}
	return nil
}

// checks that self-hosted etcd pods are scheduled on different master nodes
// when possible
func checkEtcdPodDistribution(c *pluton.Cluster, etcdClusterSize int) error {
	// check that number of unique nodes etcd pods run on is equal to the
	// lesser value betweeen total number of master nodes and total number
	// of etcd pods
	out, err := c.Kubectl(`get po -l etcd_cluster=kube-etcd -o jsonpath='{.items[*].status.hostIP}' --namespace=kube-system`)
	if err != nil {
		return err
	}

	nodeIPs := strings.Split(out, " ")
	nodeSet := map[string]struct{}{}
	for _, node := range nodeIPs {
		nodeSet[node] = struct{}{}
	}

	var expectedUniqueNodes int
	if len(c.Masters) > etcdClusterSize {
		expectedUniqueNodes = etcdClusterSize
	} else {
		expectedUniqueNodes = len(c.Masters)
	}

	if len(nodeSet) != expectedUniqueNodes {
		return fmt.Errorf("self-hosted etcd pods not properly distributed")
	}

	// check that each node in nodeSet is a master node
	masterSet := map[string]struct{}{}
	for _, m := range c.Masters {
		masterSet[m.PrivateIP()] = struct{}{}
	}

	for k := range nodeSet {
		if _, ok := masterSet[k]; !ok {
			return fmt.Errorf("detected self-hosted etcd pod running on non-master node %v %v", masterSet, nodeSet)
		}
	}

	return nil
}
