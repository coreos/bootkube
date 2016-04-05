package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/coreos/bootkube/pkg/assets"
)

var (
	cmdRender = &cobra.Command{
		Use:     "render",
		Short:   "Render default cluster manifests",
		Long:    "",
		PreRunE: validateRenderOpts,
		RunE:    runCmdRender,
	}

	outDir string
)

func init() {
	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&outDir, "outdir", "", "Output path for rendered manifests")
}

func runCmdRender(cmd *cobra.Command, args []string) error {
	as := assets.Assets{}

	// TLS assets
	tlsAssets, err := assets.NewTLSAssets()
	if err != nil {
		return err
	}
	as = append(as, tlsAssets...)

	// Token Auth
	as = append(as, assets.NewTokenAuth())

	// K8S kubeconfig
	kubeConfig, err := assets.NewKubeConfig(as)
	if err != nil {
		return err
	}
	as = append(as, kubeConfig)

	// K8S APIServer secret
	apiSecret, err := assets.NewAPIServerSecret(as)
	if err != nil {
		return err
	}
	as = append(as, apiSecret)

	// K8S ControllerManager secret
	cmSecret, err := assets.NewControllerManagerSecret(as)
	if err != nil {
		return err
	}
	as = append(as, cmSecret)

	// Static Assets
	as = append(as, assets.StaticAssets()...)

	return as.WriteFiles(outDir)
}

func validateRenderOpts(cmd *cobra.Command, args []string) error {
	if outDir == "" {
		return errors.New("Missing required flag: --outdir")
	}
	return nil
}
