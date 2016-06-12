package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/coreos/bootkube/pkg/version"
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

func init() {
	remove := pflag.Lookup("log-flush-frequency")
	remove.Hidden = true
}

func main() {
	cmdRoot.AddCommand(cmdVersion)
	if err := cmdRoot.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
