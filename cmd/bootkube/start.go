package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/kubernetes-incubator/bootkube/pkg/bootkube"
)

var (
	cmdStart = &cobra.Command{
		Use:          "start",
		Short:        "Start the bootkube service",
		Long:         "",
		PreRunE:      validateStartOpts,
		RunE:         runCmdStart,
		SilenceUsage: true,
	}

	startOpts struct {
		assetDir        string
		podManifestPath string
	}
)

func init() {
	cmdRoot.AddCommand(cmdStart)
	cmdStart.Flags().StringVar(&startOpts.assetDir, "asset-dir", "", "Path to the cluster asset directory. Expected layout genereted by the `bootkube render` command.")
	cmdStart.Flags().StringVar(&startOpts.podManifestPath, "pod-manifest-path", "/etc/kubernetes/manifests", "The location where the kubelet is configured to look for static pod manifests.")
}

func runCmdStart(cmd *cobra.Command, args []string) error {
	bk, err := bootkube.NewBootkube(bootkube.Config{
		AssetDir:        startOpts.assetDir,
		PodManifestPath: startOpts.podManifestPath,
	})
	if err != nil {
		return err
	}

	err = bk.Run()
	if err != nil {
		// Always report errors.
		bootkube.UserOutput("Error: %v\n", err)
	}
	return err
}

func validateStartOpts(cmd *cobra.Command, args []string) error {
	if startOpts.podManifestPath == "" {
		return errors.New("missing required flag: --pod-manifest-path")
	}
	if startOpts.assetDir == "" {
		return errors.New("missing required flag: --asset-dir")
	}
	return nil
}
