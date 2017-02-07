package bootkube

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api/v1"
	certificates "k8s.io/kubernetes/pkg/apis/certificates/v1alpha1"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5/typed/certificates/v1alpha1"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util/wait"
)

// ApproveKubeletCSRs indefinitely approves any CSRs submitted by Kubelet
// instances, that are in the process of bootstrapping their TLS assets, without
// making any kind of validation.
func ApproveKubeletCSRs() error {
	errCh := make(chan error)

	client, err := clientset.NewForConfig(
		&restclient.Config{Host: insecureAPIAddr},
	)
	if err != nil {
		return err
	}
	csrClient := client.CertificatesV1alpha1Client.CertificateSigningRequests()

	watchList := cache.NewListWatchFromClient(
		client.CertificatesV1alpha1().RESTClient(),
		"certificatesigningrequests",
		v1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchList,
		&certificates.CertificateSigningRequest{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if request, ok := obj.(*certificates.CertificateSigningRequest); ok {
					approveCSR(csrClient, request, errCh)
				}
			},
			UpdateFunc: func(_, obj interface{}) {
				if request, ok := obj.(*certificates.CertificateSigningRequest); ok {
					approveCSR(csrClient, request, errCh)
				}
			},
		},
	)

	go controller.Run(wait.NeverStop)
	return <-errCh
}

func approveCSR(client v1alpha1.CertificateSigningRequestInterface, request *certificates.CertificateSigningRequest, errCh chan error) {
	condition := certificates.CertificateSigningRequestCondition{
		Type:    certificates.CertificateApproved,
		Reason:  "AutoApproved",
		Message: "Auto approving of all kubelet CSRs is enabled on bootkube",
	}

	for {
		// Verify that the CSR hasn't been approved or denied already.
		//
		// There are only two possible conditions (CertificateApproved and
		// CertificateDenied). Therefore if the CSR already has a condition,
		// it means that the request has already been approved or denied, and that
		// we should ignore the request.
		if len(request.Status.Conditions) > 0 {
			return
		}

		// Approve the CSR.
		request.Status.Conditions = append(request.Status.Conditions, condition)

		// Submit the updated CSR.
		if _, err := client.UpdateApproval(request); err != nil {
			if strings.Contains(err.Error(), "the object has been modified") {
				// The CSR might have been updated by a third-party, retry until we
				// succeed.
				request, err = client.Get(request.Name)
				if err != nil {
					errCh <- fmt.Errorf("Error retrieving Kubelet's CSR %q: %s", request.Name, err)
					return
				}
				continue
			}

			errCh <- fmt.Errorf("Error approving Kubelet's CSR %q: %s", request.Name, err)
			return
		}

		UserOutput("Approved Kubelet's CSR %q\n", request.Name)
		return
	}
}
