package main

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kubernetes-incubator/bootkube/pkg/asset"
	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
)

const (
	apiOffset            = 1
	dnsOffset            = 10
	defaultServiceBaseIP = "10.3.0.0"
	defaultEtcdServers   = "https://127.0.0.1:2379"
)

var (
	cmdRender = &cobra.Command{
		Use:          "render",
		Short:        "Render default cluster manifests",
		Long:         "",
		PreRunE:      validateRenderOpts,
		RunE:         runCmdRender,
		SilenceUsage: true,
	}

	renderOpts struct {
		assetDir            string
		caCertificatePath   string
		caPrivateKeyPath    string
		etcdCAPath          string
		etcdCertificatePath string
		etcdPrivateKeyPath  string
		etcdServers         string
		apiServers          string
		altNames            string
		podCIDR             string
		serviceCIDR         string
		cloudProvider       string
		networkProvider     string
	}

	imageVersions = asset.DefaultImages
)

func init() {
	cmdRoot.AddCommand(cmdRender)
	cmdRender.Flags().StringVar(&renderOpts.assetDir, "asset-dir", "", "Output path for rendered assets")
	cmdRender.Flags().StringVar(&renderOpts.caCertificatePath, "ca-certificate-path", "", "Path to an existing PEM encoded CA. If provided, TLS assets will be generated using this certificate authority.")
	cmdRender.Flags().StringVar(&renderOpts.caPrivateKeyPath, "ca-private-key-path", "", "Path to an existing Certificate Authority RSA private key. Required if --ca-certificate is set.")
	cmdRender.Flags().StringVar(&renderOpts.etcdCAPath, "etcd-ca-path", "", "Path to an existing PEM encoded CA that will be used for TLS-enabled communication between the apiserver and etcd. Must be used in conjunction with --etcd-certificate-path and --etcd-private-key-path, and must have etcd configured to use TLS with matching secrets.")
	cmdRender.Flags().StringVar(&renderOpts.etcdCertificatePath, "etcd-certificate-path", "", "Path to an existing certificate that will be used for TLS-enabled communication between the apiserver and etcd. Must be used in conjunction with --etcd-ca-path and --etcd-private-key-path, and must have etcd configured to use TLS with matching secrets.")
	cmdRender.Flags().StringVar(&renderOpts.etcdPrivateKeyPath, "etcd-private-key-path", "", "Path to an existing private key that will be used for TLS-enabled communication between the apiserver and etcd. Must be used in conjunction with --etcd-ca-path and --etcd-certificate-path, and must have etcd configured to use TLS with matching secrets.")
	cmdRender.Flags().StringVar(&renderOpts.etcdServers, "etcd-servers", defaultEtcdServers, "List of etcd servers URLs including host:port, comma separated")
	cmdRender.Flags().StringVar(&renderOpts.apiServers, "api-servers", "https://127.0.0.1:6443", "List of API server URLs including host:port, commma seprated")
	cmdRender.Flags().StringVar(&renderOpts.altNames, "api-server-alt-names", "", "List of SANs to use in api-server certificate. Example: 'IP=127.0.0.1,IP=127.0.0.2,DNS=localhost'. If empty, SANs will be extracted from the --api-servers flag.")
	cmdRender.Flags().StringVar(&renderOpts.podCIDR, "pod-cidr", "10.2.0.0/16", "The CIDR range of cluster pods.")
	cmdRender.Flags().StringVar(&renderOpts.serviceCIDR, "service-cidr", "10.3.0.0/24", "The CIDR range of cluster services.")
	cmdRender.Flags().StringVar(&renderOpts.cloudProvider, "cloud-provider", "", "The provider for cloud services.  Empty string for no provider")
	cmdRender.Flags().StringVar(&renderOpts.networkProvider, "network-provider", "flannel", "CNI network provider (flannel or experimental-canal).")
}

func runCmdRender(cmd *cobra.Command, args []string) error {
	config, err := flagsToAssetConfig()
	if err != nil {
		return err
	}

	as, err := asset.NewDefaultAssets(*config)
	if err != nil {
		return err
	}

	return as.WriteFiles(renderOpts.assetDir)
}

func validateRenderOpts(cmd *cobra.Command, args []string) error {
	if renderOpts.caCertificatePath != "" && renderOpts.caPrivateKeyPath == "" {
		return errors.New("You must provide the --ca-private-key-path flag when --ca-certificate-path is provided.")
	}
	if renderOpts.caPrivateKeyPath != "" && renderOpts.caCertificatePath == "" {
		return errors.New("You must provide the --ca-certificate-path flag when --ca-private-key-path is provided.")
	}
	if (renderOpts.etcdCAPath != "" || renderOpts.etcdCertificatePath != "" || renderOpts.etcdPrivateKeyPath != "") && (renderOpts.etcdCAPath == "" || renderOpts.etcdCertificatePath == "" || renderOpts.etcdPrivateKeyPath == "") {
		return errors.New("You must specify either all or none of --etcd-ca-path, --etcd-certificate-path, and --etcd-private-key-path")
	}
	if renderOpts.assetDir == "" {
		return errors.New("Missing required flag: --asset-dir")
	}
	if renderOpts.etcdServers == "" {
		return errors.New("Missing required flag: --etcd-servers")
	}
	if renderOpts.apiServers == "" {
		return errors.New("Missing requried flag: --api-servers")
	}
	if renderOpts.networkProvider != asset.NetworkFlannel && renderOpts.networkProvider != asset.NetworkCalico && renderOpts.networkProvider != asset.NetworkCanal {
		return errors.New("Must specify --network-provider flannel or experimental-calico or experimental-canal")
	}
	return nil
}

func flagsToAssetConfig() (c *asset.Config, err error) {
	apiServers, err := parseURLs(renderOpts.apiServers)
	if err != nil {
		return nil, err
	}
	altNames, err := parseAltNames(renderOpts.altNames)
	if err != nil {
		return nil, err
	}
	if altNames == nil {
		// Fall back to parsing from api-server list
		altNames = altNamesFromURLs(apiServers)
	}

	var caCert *x509.Certificate
	var caPrivKey *rsa.PrivateKey
	if renderOpts.caCertificatePath != "" {
		caPrivKey, caCert, err = parseCertAndPrivateKeyFromDisk(renderOpts.caCertificatePath, renderOpts.caPrivateKeyPath)
		if err != nil {
			return nil, err
		}
	}

	_, podNet, err := net.ParseCIDR(renderOpts.podCIDR)
	if err != nil {
		return nil, err
	}

	_, serviceNet, err := net.ParseCIDR(renderOpts.serviceCIDR)
	if err != nil {
		return nil, err
	}

	if podNet.Contains(serviceNet.IP) || serviceNet.Contains(podNet.IP) {
		return nil, fmt.Errorf("Pod CIDR %s and service CIDR %s must not overlap", podNet.String(), serviceNet.String())
	}

	apiServiceIP, err := offsetServiceIP(serviceNet, apiOffset)
	if err != nil {
		return nil, err
	}

	dnsServiceIP, err := offsetServiceIP(serviceNet, dnsOffset)
	if err != nil {
		return nil, err
	}

	etcdServers, err := parseURLs(renderOpts.etcdServers)
	if err != nil {
		return nil, err
	}

	etcdUseTLS := false
	for _, url := range etcdServers {
		if url.Scheme == "https" {
			etcdUseTLS = true
		}
	}

	var etcdCACert *x509.Certificate
	if renderOpts.etcdCAPath != "" {
		etcdCACert, err = parseCertFromDisk(renderOpts.etcdCAPath)
		if err != nil {
			return nil, err
		}
	}
	var etcdClientCert *x509.Certificate
	var etcdClientKey *rsa.PrivateKey
	if renderOpts.etcdCertificatePath != "" {
		etcdClientKey, etcdClientCert, err = parseCertAndPrivateKeyFromDisk(renderOpts.etcdCertificatePath, renderOpts.etcdPrivateKeyPath)
		if err != nil {
			return nil, err
		}
	}

	// TODO: Find better option than asking users to make manual changes
	if serviceNet.IP.String() != defaultServiceBaseIP {
		fmt.Printf("You have selected a non-default service CIDR %s - be sure your kubelet service file uses --cluster-dns=%s\n", serviceNet.String(), dnsServiceIP.String())
	}

	return &asset.Config{
		EtcdCACert:      etcdCACert,
		EtcdClientCert:  etcdClientCert,
		EtcdClientKey:   etcdClientKey,
		EtcdServers:     etcdServers,
		EtcdUseTLS:      etcdUseTLS,
		CACert:          caCert,
		CAPrivKey:       caPrivKey,
		APIServers:      apiServers,
		AltNames:        altNames,
		PodCIDR:         podNet,
		ServiceCIDR:     serviceNet,
		APIServiceIP:    apiServiceIP,
		DNSServiceIP:    dnsServiceIP,
		CloudProvider:   renderOpts.cloudProvider,
		NetworkProvider: renderOpts.networkProvider,
		Images:          imageVersions,
	}, nil
}

func parseCertAndPrivateKeyFromDisk(caCertPath, privKeyPath string) (*rsa.PrivateKey, *x509.Certificate, error) {
	// Parse CA Private key.
	keypem, err := ioutil.ReadFile(privKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading ca private key file at %s: %v", privKeyPath, err)
	}
	key, err := tlsutil.ParsePEMEncodedPrivateKey(keypem)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse CA private key: %v", err)
	}
	// Parse CA Cert.
	cert, err := parseCertFromDisk(caCertPath)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}

func parseCertFromDisk(caCertPath string) (*x509.Certificate, error) {
	capem, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("error reading ca cert file at %s: %v", caCertPath, err)
	}
	cert, err := tlsutil.ParsePEMEncodedCACert(capem)
	if err != nil {
		return nil, fmt.Errorf("unable to parse CA Cert: %v", err)
	}
	return cert, nil
}

func parseURLs(s string) ([]*url.URL, error) {
	var out []*url.URL
	for _, u := range strings.Split(s, ",") {
		parsed, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}

func parseAltNames(s string) (*tlsutil.AltNames, error) {
	if s == "" {
		return nil, nil
	}
	var alt tlsutil.AltNames
	for _, an := range strings.Split(s, ",") {
		switch {
		case strings.HasPrefix(an, "DNS="):
			alt.DNSNames = append(alt.DNSNames, strings.TrimPrefix(an, "DNS="))
		case strings.HasPrefix(an, "IP="):
			ip := net.ParseIP(strings.TrimPrefix(an, "IP="))
			if ip == nil {
				return nil, fmt.Errorf("Invalid IP alt name: %s", an)
			}
			alt.IPs = append(alt.IPs, ip)
		default:
			return nil, fmt.Errorf("Invalid alt name: %s", an)
		}
	}
	return &alt, nil
}

func altNamesFromURLs(urls []*url.URL) *tlsutil.AltNames {
	var an tlsutil.AltNames
	for _, u := range urls {
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			host = u.Host
		}
		ip := net.ParseIP(host)
		if ip == nil {
			an.DNSNames = append(an.DNSNames, host)
		} else {
			an.IPs = append(an.IPs, ip)
		}
	}
	return &an
}

// offsetServiceIP returns an IP offset by up to 255.
// TODO: do numeric conversion to generalize this utility.
func offsetServiceIP(ipnet *net.IPNet, offset int) (net.IP, error) {
	ip := make(net.IP, len(ipnet.IP))
	copy(ip, ipnet.IP)
	for i := 0; i < offset; i++ {
		incIPv4(ip)
	}
	if ipnet.Contains(ip) {
		return ip, nil
	}
	return net.IP([]byte("")), fmt.Errorf("Service IP %v is not in %s", ip, ipnet)
}

func incIPv4(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
