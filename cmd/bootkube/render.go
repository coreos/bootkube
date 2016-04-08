package main

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/coreos/bootkube/pkg/asset"
)

var (
	cmdRender = &cobra.Command{
		Use:     "render",
		Short:   "Render default cluster manifests",
		Long:    "",
		PreRunE: validateRenderOpts,
		RunE:    runCmdRender,
	}

	outDir    string
	assetConf asset.Config
)

func init() {
	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&outDir, "outdir", "", "Output path for rendered manifests")
	cmdRender.Flags().StringVar(&assetConf.APIServerCertIPAddrs, "apiserver-cert-ip-addrs", "", "IP Addresses to include in API Server cert SANs")
	cmdRender.Flags().StringVar(&assetConf.ETCDServers, "etcd-servers", "", "List of etcd servers to contact")
	cmdRender.Flags().StringVar(&assetConf.APIServers, "api-servers", "", "List of api servers to contact")
}

func runCmdRender(cmd *cobra.Command, args []string) error {
	as, err := asset.NewDefaultAssets(assetConf)
	if err != nil {
		return err
	}
	return as.WriteFiles(outDir)
}

func validateRenderOpts(cmd *cobra.Command, args []string) error {
	if outDir == "" {
		return errors.New("Missing required flag: --outdir")
	}
	if assetConf.APIServerCertIPAddrs == "" {
		return errors.New("Missing required flag: --apiserver-cert-ip-addrs")
	}
	if assetConf.APIServers == "" {
		return errors.New("Missing required flag: --api-servers")
	}
	if assetConf.ETCDServers == "" {
		return errors.New("Missing required flag: --etcd-servers")
	}
	return nil
}
