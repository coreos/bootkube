package asset

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/kubernetes-incubator/bootkube/pkg/tlsutil"
)

func newTLSAssets(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey, altNames tlsutil.AltNames) ([]Asset, error) {
	var (
		assets []Asset
		err    error
	)

	if caCert == nil {
		caPrivKey, caCert, err = newCACert()
		if err != nil {
			return assets, err
		}
	}

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

	adminKey, adminCert, err := newAdminKeyAndCert(caCert, caPrivKey)
	if err != nil {
		return assets, err
	}

	bootstrapToken, err := newBootstrapAuthToken()
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
		{Name: AssetPathAdminKey, Data: tlsutil.EncodePrivateKeyPEM(adminKey)},
		{Name: AssetPathAdminCert, Data: tlsutil.EncodeCertificatePEM(adminCert)},
		{Name: AssetPathBootstrapAuthToken, Data: bootstrapToken},
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
		Organization: []string{"kube-aws"},
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
	altNames.IPs = append(altNames.IPs, net.ParseIP("10.3.0.1"))
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

func newAdminKeyAndCert(caCert *x509.Certificate, caPrivKey *rsa.PrivateKey) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}
	config := tlsutil.CertConfig{
		CommonName:   "cluster-admin",
		Organization: []string{"system:masters"},
	}
	cert, err := tlsutil.NewSignedCertificate(config, key, caCert, caPrivKey)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, err
}

// newBootstrapAuthToken creates a static token, containing a single
// randomly-generated bearer token mapped to the user and group
// 'kubelet-bootstrap', as documented in k8s.io/docs/admin/authentication/.
//
// This auth token is meant to be used by kubelet to bootstrap its TLS
// certificate: k8s.io/docs/admin/kubelet-tls-bootstrapping.
func newBootstrapAuthToken() ([]byte, error) {
	// We must create at least 128 bits of entropy from a secure source.
	bearer, err := generateRandomString(16)
	if err != nil {
		return nil, err
	}

	token := fmt.Sprintf("%s,kubelet-bootstrap,10001,\"system:kubelet-bootstrap\"\n", bearer)
	return []byte(token), nil
}

// parseBootstrapAuthToken retrieves the first bearer token from the
// given data of a static token file.
func parseBootstrapAuthToken(data []byte) (string, error) {
	// The format is: token,user,uid,"group1,group2,group3".
	sp := strings.Split(string(data), ",")
	if len(sp) < 3 {
		return "", errors.New("could not extract bearer token: invalid format")
	}
	return sp[0], nil
}

func generateRandomString(len int) (string, error) {
	b := make([]byte, len)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
