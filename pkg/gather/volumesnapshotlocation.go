package gather

import (
	"context"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func AllVolumeSnapshotLocations(clusterClient client.Client) (*velerov1.VolumeSnapshotLocationList, error) {
	volumeSnapshotLocation := &velerov1.VolumeSnapshotLocationList{}
	err := clusterClient.List(context.Background(), volumeSnapshotLocation)
	if err != nil {
		return nil, err
	}
	if len(volumeSnapshotLocation.Items) == 0 {
		// just warning, do not return error
		return nil, nil
	}
	return volumeSnapshotLocation, nil
}
