package asset

import (
	"crypto/rsa"
	"crypto/x509"
	"net"
	"net/url"
	"strings"

	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
)

func newTLSAssets(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) ([]Asset, error) {
	var (
		assets []Asset
		err    error
	)

	apiKey, apiCert, err := newAPIKeyAndCert(caCert, caPrivKey, altNames)
	if err != nil {
		return assets, err
	}

	saPrivKey, err := tlsutil.NewPrivateKey()
	if err != nil {
		return assets, err
	}

	saPubKey, err := tlsutil.EncodePublicKeyPEM(&saPrivKey.PublicKey)
	if err != nil {
		return assets, err
	}

	kubeletKey, kubeletCert, err := newKubeletKeyAndCert(caCert, caPrivKey)
	if err != nil {
		return assets, err
	}

	assets = append(assets, []Asset{
		{Name: AssetPathCAKey, Data: tlsutil.EncodePrivateKeyPEM(caPrivKey)},
		{Name: AssetPathCACert, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathAPIServerKey, Data: tlsutil.EncodePrivateKeyPEM(apiKey)},
		{Name: AssetPathAPIServerCert, Data: tlsutil.EncodeCertificatePEM(apiCert)},
		{Name: AssetPathServiceAccountPrivKey, Data: tlsutil.EncodePrivateKeyPEM(saPrivKey)},
		{Name: AssetPathServiceAccountPubKey, Data: saPubKey},
		{Name: AssetPathKubeletKey, Data: tlsutil.EncodePrivateKeyPEM(kubeletKey)},
		{Name: AssetPathKubeletCert, Data: tlsutil.EncodeCertificatePEM(kubeletCert)},
	}...)
	return assets, nil
}

func newCACert() (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	config := tlsutil.CertConfig{
		CommonName:   "kube-ca",
		Organization: []string{"bootkube"},
	}

	cert, err := tlsutil.NewSelfSignedCACertificate(config, key)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, err
}

func newAPIKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	altNames.DNSNames = append(altNames.DNSNames, []string{
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		"kubernetes.default.svc.cluster.local",
	}...)

	config := tlsutil.CertConfig{
		CommonName:   "kube-apiserver",
		Organization: []string{"kube-master"},
		AltNames:     altNames,
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

func newKubeletKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey) (*rsa.PrivateKey, *x509.Certificate, error) {
	// TLS organizations map to Kubernetes groups, and "system:masters"
	// is a well-known Kubernetes group that gives a user admin power.
	//
	// For now, put the kubelets in this group. Later we can restrict
	// their credentials, likely with the help of TLS bootstrapping.
	const orgSystemMasters = "system:masters"

	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	config := tlsutil.CertConfig{
		CommonName:   "kubelet",
		Organization: []string{orgSystemMasters},
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

func newEtcdTLSAssets(etcdCACert, etcdClientCert *x509.Certificate, etcdClientKey *rsa.PrivateKey, caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, etcdServers []*url.URL) ([]Asset, error) {
	var assets []Asset
	if etcdCACert == nil {
		// Use the master CA to generate etcd assets.
		etcdCACert = caCert

		// Create an etcd client cert.
		var err error
		etcdClientKey, etcdClientCert, err = newEtcdKeyAndCert(caCert, caPrivKey, "etcd-client", etcdServers)
		if err != nil {
			return nil, err
		}

		// Create an etcd peer cert (not consumed by self-hosted components).
		etcdPeerKey, etcdPeerCert, err := newEtcdKeyAndCert(caCert, caPrivKey, "etcd-peer", etcdServers)
		if err != nil {
			return nil, err
		}
		assets = append(assets, []Asset{
			{Name: AssetPathEtcdPeerKey, Data: tlsutil.EncodePrivateKeyPEM(etcdPeerKey)},
			{Name: AssetPathEtcdPeerCert, Data: tlsutil.EncodeCertificatePEM(etcdPeerCert)},
		}...)
	}

	assets = append(assets, []Asset{
		{Name: AssetPathEtcdCA, Data: tlsutil.EncodeCertificatePEM(etcdCACert)},
		{Name: AssetPathEtcdClientKey, Data: tlsutil.EncodePrivateKeyPEM(etcdClientKey)},
		{Name: AssetPathEtcdClientCert, Data: tlsutil.EncodeCertificatePEM(etcdClientCert)},
	}...)

	return assets, nil
}

// newSelfHostedEtcdTLSAssets automatically generates three suites of x509 certificates (CA, key, cert)
// for self-hosted etcd related components. Two suites are used by etcd members' client and peer ports;
// one is used via etcd client to talk to etcd by operator, apiserver.
// Self-hosted etcd doesn't allow user to specify etcd certs.
func newSelfHostedEtcdTLSAssets(etcdSvcIP, bootEtcdSvcIP string, caCert *x509.Certificate, caPrivKey *rsa.PrivateKey) (Assets, error) {
	// TODO: This method uses tlsutil.NewSignedCertificate() which will create certs for both client and server auth.
	//       We can limit on finer granularity.

	var assets []Asset

	key, cert, err := newKeyAndCert(caCert, caPrivKey, "etcd member client", []string{
		etcdSvcIP,
		bootEtcdSvcIP,
		"127.0.0.1",
		"localhost",
		"*.kube-etcd.kube-system.svc.cluster.local",
		"kube-etcd-client.kube-system.svc.cluster.local",
	})
	if err != nil {
		return nil, err
	}
	assets = append(assets, []Asset{
		{Name: AssetPathSelfHostedEtcdMemberClientCA, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathSelfHostedEtcdMemberClientKey, Data: tlsutil.EncodePrivateKeyPEM(key)},
		{Name: AssetPathSelfHostedEtcdMemberClientCert, Data: tlsutil.EncodeCertificatePEM(cert)},
	}...)

	key, cert, err = newKeyAndCert(caCert, caPrivKey, "etcd member peer", []string{
		bootEtcdSvcIP,
		"*.kube-etcd.kube-system.svc.cluster.local",
		"kube-etcd-client.kube-system.svc.cluster.local",
	})
	if err != nil {
		return nil, err
	}
	assets = append(assets, []Asset{
		{Name: AssetPathSelfHostedEtcdMemberPeerCA, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathSelfHostedEtcdMemberPeerKey, Data: tlsutil.EncodePrivateKeyPEM(key)},
		{Name: AssetPathSelfHostedEtcdMemberPeerCert, Data: tlsutil.EncodeCertificatePEM(cert)},
	}...)

	key, cert, err = newKeyAndCert(caCert, caPrivKey, "operator etcd client", nil)
	if err != nil {
		return nil, err
	}
	assets = append(assets, []Asset{
		{Name: AssetPathSelfHostedOperatorEtcdCA, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathSelfHostedOperatorEtcdKey, Data: tlsutil.EncodePrivateKeyPEM(key)},
		{Name: AssetPathSelfHostedOperatorEtcdCert, Data: tlsutil.EncodeCertificatePEM(cert)},
	}...)
	// for APIServer
	assets = append(assets, []Asset{
		{Name: AssetPathEtcdCA, Data: tlsutil.EncodeCertificatePEM(caCert)},
		{Name: AssetPathEtcdClientKey, Data: tlsutil.EncodePrivateKeyPEM(key)},
		{Name: AssetPathEtcdClientCert, Data: tlsutil.EncodeCertificatePEM(cert)},
	}...)

	return assets, nil
}

func newEtcdKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, commonName string, etcdServers []*url.URL) (*rsa.PrivateKey, *x509.Certificate, error) {
	addrs := make([]string, len(etcdServers))
	for i := range etcdServers {
		addrs[i] = etcdServers[i].Host
	}
	return newKeyAndCert(caCert, caPrivKey, commonName, addrs)
}

func newKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, commonName string, addrs []string) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	var altNames tlsutil.AltNames
	for _, addr := range addrs {
		hostname := stripPort(addr)
		if ip := net.ParseIP(hostname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else {
			altNames.DNSNames = append(altNames.DNSNames, hostname)
		}
	}
	config := tlsutil.CertConfig{
		CommonName:   commonName,
		Organization: []string{"etcd"},
		AltNames:     altNames,
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

// TODO(diegs): remove this and switch to URL.Hostname() once bootkube uses Go 1.8.
func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}
