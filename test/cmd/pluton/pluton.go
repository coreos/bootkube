// Copyright 2015 CoreOS, Inc.
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

package main

import (
	"fmt"
	"os"
	"sort"

	// register test set
	_ "github.com/kubernetes-incubator/bootkube/test"

	"github.com/coreos/mantle/cli"
	"github.com/coreos/mantle/pluton/harness"
	"github.com/spf13/cobra"
)

var (
	root = &cobra.Command{
		Use:   "pluton [command]",
		Short: "The Kubernetes Tester Based on Kola",
		//https://en.wikipedia.org/wiki/Pluton
	}

	cmdRun = &cobra.Command{
		Use:   "run [glob pattern]",
		Short: "Run run pluton tests by category",
		Long:  "run all pluton tests (default) or related groups",
		Run:   runRun,
	}

	cmdList = &cobra.Command{
		Use:   "list",
		Short: "List pluton test names",
		Run:   runList,
	}
)

func main() {
	cli.Execute(root)
}

func runRun(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		fmt.Fprintf(os.Stderr, "Extra arguements specified. Usage: 'pluton run [glob pattern]'\n")
		os.Exit(2)
	}
	var pattern string
	if len(args) == 1 {
		pattern = args[0]
	} else {
		pattern = "*" // run all tests by default
	}

	harness.RunSuite(pattern)
}

func runList(cmd *cobra.Command, args []string) {
	var list []string

	for name := range harness.Tests {
		list = append(list, name)
	}

	sort.Strings(list)

	fmt.Println("Test Name")
	for _, name := range list {
		fmt.Printf("%v\n", name)
	}
}
