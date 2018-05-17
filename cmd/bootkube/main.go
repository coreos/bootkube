package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/kubernetes-incubator/bootkube/pkg/util"
	"github.com/kubernetes-incubator/bootkube/pkg/version"
)

var (
	cmdRoot = &cobra.Command{
		Use:           "bootkube",
		Short:         "Bootkube!",
		SilenceErrors: true, // suppress cobra errors so we can handle them (also applies to subcommands)
		Long:          "",
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	flag.Parse()

	logsExit := make(chan error, 1)
	util.InitLogs(ctx, logsExit)

	cmdRoot.AddCommand(cmdVersion)
	if err := cmdRoot.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cancel()
		<-logsExit
		os.Exit(1)
	}
}
