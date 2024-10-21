package gather

import (
	"context"

	oadpv1alpha1 "github.com/openshift/oadp-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AllDataProtectionApplications(clusterClient client.Client) (*oadpv1alpha1.DataProtectionApplicationList, error) {
	dataProtectionApplicationList := &oadpv1alpha1.DataProtectionApplicationList{}
	err := clusterClient.List(context.Background(), dataProtectionApplicationList)
	if err != nil {
		return nil, err
	}
	if len(dataProtectionApplicationList.Items) == 0 {
		// just warning, do not return error
		return nil, nil
	}
	return dataProtectionApplicationList, nil
}
