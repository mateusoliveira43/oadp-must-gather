package gather

import (
	"context"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AllBackupStorageLocations(clusterClient client.Client) (*velerov1.BackupStorageLocationList, error) {
	backupStorageLocation := &velerov1.BackupStorageLocationList{}
	err := clusterClient.List(context.Background(), backupStorageLocation)
	if err != nil {
		return nil, err
	}
	if len(backupStorageLocation.Items) == 0 {
		// just warning, do not return error
		return nil, nil
	}
	return backupStorageLocation, nil
}
