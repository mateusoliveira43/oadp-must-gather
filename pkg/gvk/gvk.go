package gvk

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	CustomResourceDefinitionGVK = schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	}
	ListGVK = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "List",
	}
	ClusterServiceVersionGVK = schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "ClusterServiceVersion",
	}
	DataProtectionApplicationGVK = schema.GroupVersionKind{
		Group:   "oadp.openshift.io",
		Version: "v1alpha1",
		Kind:    "DataProtectionApplication",
	}
	BackupStorageLocationGVK = schema.GroupVersionKind{
		Group:   "velero.io",
		Version: "v1",
		Kind:    "BackupStorageLocation",
	}
	VolumeSnapshotLocationGVK = schema.GroupVersionKind{
		Group:   "velero.io",
		Version: "v1",
		Kind:    "VolumeSnapshotLocation",
	}
	StorageClassGVK = schema.GroupVersionKind{
		Group:   "storage.k8s.io",
		Version: "v1",
		Kind:    "StorageClass",
	}
)
