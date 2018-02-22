package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubernetes-incubator/bootkube/pkg/util"
	"github.com/kubernetes-incubator/bootkube/pkg/version"
)

var (
	cmdRoot = &cobra.Command{
		Use:   "bootkube",
		Short: "Bootkube!",
		Long:  "",
	}

	cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Output version information",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Version: %s\n", version.Version)
			return nil
		},
	}
)

func main() {
	flag.Parse()
	util.InitLogs()
	defer util.FlushLogs()

	cmdRoot.AddCommand(cmdVersion)
	if err := cmdRoot.Execute(); err != nil {
		os.Exit(1)
	}
}
