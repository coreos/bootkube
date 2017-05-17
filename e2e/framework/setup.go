package framework

import (
	"flag"
	"fmt"
	"os"
)

var (
	kubeconfig    = flag.String("kubeconfig", "../hack/quickstart/cluster/auth/kubeconfig", "absolute path to the kubeconfig file")
	expectedNodes = flag.Int("nodes", 0, "the number of nodes to expect")
)

func init() {
	flag.Parse()
	if *expectedNodes == 0 {
		fmt.Println("Please set --nodes flag with the number of nodes in the running cluster")
		os.Exit(1)
	}
}
