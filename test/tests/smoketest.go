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
	"bytes"
	"fmt"
	"time"

	"github.com/coreos/mantle/pluton"
	"github.com/coreos/mantle/util"
)

func Smoke(c *pluton.Cluster) {
	// run an nginx deployment and ping it
	if err := nginxCheck(c); err != nil {
		c.Fatalf("nginxCheck: %s", err)
	}
	// TODO add more basic or regression tests here
}

func nginxCheck(c *pluton.Cluster) error {
	// start nginx deployment
	_, err := c.Kubectl("run my-nginx --image=nginx --replicas=2 --port=80")
	if err != nil {
		return fmt.Errorf("starting deployment: %v", err)
	}

	// expose nginx
	_, err = c.Kubectl("expose deployment my-nginx --port=80 --type=LoadBalancer")
	if err != nil {
		return fmt.Errorf("expose deployment: %v", err)
	}
	serviceIP, err := c.Kubectl("get service my-nginx --template={{.spec.clusterIP}}")
	if err != nil {
		return fmt.Errorf("get service IP: %v", err)
	}

	// curl for welcome message
	nginxRunning := func() error {
		out, err := c.Masters[0].SSH("curl --silent " + serviceIP + ":80")
		if err != nil || !bytes.Contains(out, []byte("Welcome to nginx!")) {
			return fmt.Errorf("unable to reach nginx: %s", out)
		}
		return nil
	}
	if err := util.Retry(20, 10*time.Second, nginxRunning); err != nil {
		return err
	}

	// delete pod
	// This part of the test has been flaking a lot in more then one test.
	// We shouldn't need a retry here, but lets try it anyway and log
	// failures here as abnormal.
	deletePod := func() error {
		_, err = c.Kubectl("delete deployment my-nginx")
		if err != nil {
			c.Logf("unexpected kubectl failure deleting deployment: %v", err)
			return fmt.Errorf("delete deployment: %v", err)
		}
		return nil
	}
	if err := util.Retry(5, 5*time.Second, deletePod); err != nil {
		return err
	}

	return nil
}
