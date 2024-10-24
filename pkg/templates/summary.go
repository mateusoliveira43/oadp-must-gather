package templates

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	oadpv1alpha1 "github.com/openshift/oadp-operator/api/v1alpha1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/rest"

	"github.com/mateusoliveira43/oadp-must-gather/pkg/gvk"
)

var (
	summaryTemplateReplacesKeys = []string{
		"MUST_GATHER_VERSION",
		"ERRORS",
		"CLUSTER_ID", "OCP_VERSION", "CLOUD", "ARCH", "CLUSTER_VERSION",
		"OADP_VERSIONS",
		"DATA_PROTECTION_APPLICATIONS",
		"BACKUP_STORAGE_LOCATIONS",
		"VOLUME_SNAPSHOT_LOCATIONS",
		// TODO NAC
		"STORAGE_CLASSES",
		"CUSTOM_RESOURCE_DEFINITION",
	}
	summaryTemplateReplaces = map[string]string{}
)

// TODO https://stackoverflow.com/a/31742265
// TODO https://github.com/kubernetes-sigs/kubebuilder/blob/master/pkg/plugins/golang/v4/scaffolds/internal/templates/readme.go
// https://deploy-preview-4185--kubebuilder.netlify.app/plugins/extending/extending_cli_features_and_plugins#example-bollerplate
// https://github.com/kubernetes-sigs/kubebuilder/tree/master/pkg/machinery
const summaryTemplate = `# OADP must-gather summary version <<MUST_GATHER_VERSION>>

## Errors

<<ERRORS>>

## Cluster information

| Cluster ID | OpenShift version | Cloud provider | Architecture |
| ---------- | ----------------- | -------------- | ------------ |
| <<CLUSTER_ID>> | <<OCP_VERSION>> | <<CLOUD>> | <<ARCH>> |

<<CLUSTER_VERSION>>

## OADP operator installation information

<<OADP_VERSIONS>>

### DataProtectionApplications (DPAs)

<<DATA_PROTECTION_APPLICATIONS>>

### BackupStorageLocations (BSLs)

<<BACKUP_STORAGE_LOCATIONS>>

### VolumeSnapshotLocations (VSLs)

<<VOLUME_SNAPSHOT_LOCATIONS>>

### Backups

TODO

### Restores

TODO

### Schedules

TODO

### BackupRepositories

TODO

### DataUploads

TODO

### DataDownloads

TODO

### PodVolumeBackups

TODO

## Available StorageClasses in cluster

<<STORAGE_CLASSES>>

## CSI VolumeSnapshotClasses

TODO Is this important for 1.3+?

## CustomResourceDefinitions

<<CUSTOM_RESOURCE_DEFINITION>>
`

func init() {
	for _, key := range summaryTemplateReplacesKeys {
		summaryTemplateReplaces[key] = ""
	}
}

func ReplaceMustGatherVersion(version string) {
	summaryTemplateReplaces["MUST_GATHER_VERSION"] = "`" + version + "`"
}

func ReplaceClusterInformationSection(path string, clusterID string, clusterVersion *openshiftconfigv1.ClusterVersion, infrastructure *openshiftconfigv1.Infrastructure, nodeList *corev1.NodeList) {
	summaryTemplateReplaces["CLUSTER_ID"] = clusterID

	if clusterVersion != nil {
		// nil check
		summaryTemplateReplaces["OCP_VERSION"] = clusterVersion.Status.Desired.Version
		summaryTemplateReplaces["CLUSTER_VERSION"] = createYAML(path, "cluster-scoped-resources/config.openshift.io/clusterversions.yaml", clusterVersion)
	} else {
		// this is code is unreachable?
		summaryTemplateReplaces["OCP_VERSION"] = "❌ error"
		summaryTemplateReplaces["OCP_CAPABILITIES"] = "❌ error"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No ClusterVersion found in cluster\n\n"
	}

	if infrastructure != nil {
		cloudProvider := string(infrastructure.Spec.PlatformSpec.Type)
		summaryTemplateReplaces["CLOUD"] = cloudProvider
	} else {
		summaryTemplateReplaces["CLOUD"] = "❌ error"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No Infrastructure found in cluster\n\n"
	}

	if nodeList != nil {
		architectureText := ""
		for _, node := range nodeList.Items {
			arch := node.Status.NodeInfo.OperatingSystem + "/" + node.Status.NodeInfo.Architecture
			if len(architectureText) == 0 {
				architectureText += arch
			} else {
				if !strings.Contains(architectureText, arch) {
					architectureText += " | " + arch
				}
			}
		}
		summaryTemplateReplaces["ARCH"] = architectureText
	} else {
		summaryTemplateReplaces["ARCH"] = "❌ error"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No Node found in cluster\n\n"
	}
	// TODO maybe nil case can be simplified by initializing everything with an error state/message
}

func ReplaceOADPOperatorInstallationSection(path string, clusterServiceVersionList *operatorsv1alpha1.ClusterServiceVersionList) {
	if clusterServiceVersionList != nil {
		oadpOperatorsText := ""
		foundOADP := false
		foundRelatedProducts := false
		importantCSVsByNamespace := map[string][]operatorsv1alpha1.ClusterServiceVersion{}

		// ?Managed Velero operator? only available in ROSA? https://github.com/openshift/managed-velero-operator
		//
		// ?IBM Fusion?
		//
		// ?Dell Power Protect?
		//
		// upstream velero?
		relatedProducts := []string{"OpenShift Virtualization", "Advanced Cluster Management for Kubernetes", "Submariner"}
		communityProducts := []string{"KubeVirt HyperConverged Cluster Operator"}

		for _, csv := range clusterServiceVersionList.Items {
			// OADP dev, community and prod operators have same spec.displayName
			if csv.Spec.DisplayName == "OADP Operator" {
				oadpOperatorsText += fmt.Sprintf("Found **%v** version **%v** installed in **%v** namespace\n\n", csv.Spec.DisplayName, csv.Spec.Version, csv.Namespace)
				foundOADP = true
				importantCSVsByNamespace[csv.Namespace] = append(importantCSVsByNamespace[csv.Namespace], csv)
			}
			if slices.Contains(relatedProducts, csv.Spec.DisplayName) {
				oadpOperatorsText += fmt.Sprintf("Found related product **%v** version **%v** installed in **%v** namespace\n\n", csv.Spec.DisplayName, csv.Spec.Version, csv.Namespace)
				foundRelatedProducts = true
				importantCSVsByNamespace[csv.Namespace] = append(importantCSVsByNamespace[csv.Namespace], csv)
			}
			if slices.Contains(communityProducts, csv.Spec.DisplayName) {
				oadpOperatorsText += fmt.Sprintf("⚠️ Found related product **%v (Community)** version **%v** installed in **%v** namespace\n\n", csv.Spec.DisplayName, csv.Spec.Version, csv.Namespace)
				foundRelatedProducts = true
				importantCSVsByNamespace[csv.Namespace] = append(importantCSVsByNamespace[csv.Namespace], csv)
			}
		}
		if len(importantCSVsByNamespace) == 0 {
			summaryTemplateReplaces["OADP_VERSIONS"] = "❌ No OADP Operator was found installed in the cluster\n\nNo related product was found installed in the cluster"
			summaryTemplateReplaces["ERRORS"] += "🚫 No OADP Operator was found installed in the cluster\n\n"
		} else {
			for namespace, csvs := range importantCSVsByNamespace {
				list := &corev1.List{}
				list.GetObjectKind().SetGroupVersionKind(gvk.ListGVK)
				for _, csv := range csvs {
					csv.GetObjectKind().SetGroupVersionKind(gvk.ClusterServiceVersionGVK)
					list.Items = append(list.Items, runtime.RawExtension{Object: &csv})
				}
				// TODO permission
				// TODO need defer somewhere?
				folder := fmt.Sprintf("namespaces/%s/operators.coreos.com/clusterserviceversions", namespace)
				err := os.MkdirAll(path+folder, 0777)
				if err != nil {
					fmt.Printf("An error happened while creating folder structure: %v\n", err)
					// TODO!!!!
					continue
				}
				oadpOperatorsText += createYAML(path, folder+"/clusterserviceversions.yaml", list)
			}
			if !foundOADP {
				summaryTemplateReplaces["OADP_VERSIONS"] += "❌ No OADP Operator was found installed in the cluster\n\n"
				summaryTemplateReplaces["ERRORS"] += "🚫 No OADP Operator was found installed in the cluster\n\n"
			}
			summaryTemplateReplaces["OADP_VERSIONS"] += oadpOperatorsText
			if !foundRelatedProducts {
				summaryTemplateReplaces["OADP_VERSIONS"] += "No related product was found installed in the cluster"
			}
		}
	} else {
		summaryTemplateReplaces["OADP_VERSIONS"] = "❌ No OADP Operator was found installed in the cluster\n\nNo related product was found installed in the cluster"
		summaryTemplateReplaces["ERRORS"] += "🚫 No ClusterServiceVersion was found in cluster\n\n"
	}
}

func ReplaceDataProtectionApplicationsSection(path string, dataProtectionApplicationList *oadpv1alpha1.DataProtectionApplicationList) {
	if dataProtectionApplicationList != nil {
		dataProtectionApplicationsByNamespace := map[string][]oadpv1alpha1.DataProtectionApplication{}

		for _, dataProtectionApplication := range dataProtectionApplicationList.Items {
			dataProtectionApplicationsByNamespace[dataProtectionApplication.Namespace] = append(dataProtectionApplicationsByNamespace[dataProtectionApplication.Namespace], dataProtectionApplication)
		}

		summaryTemplateReplaces["DATA_PROTECTION_APPLICATIONS"] += "| Namespace | Name | spec.unsupportedOverrides | status.conditions[0] | details |\n| --- | --- | --- | --- | --- |\n"
		for namespace, dataProtectionApplications := range dataProtectionApplicationsByNamespace {
			list := &corev1.List{}
			list.GetObjectKind().SetGroupVersionKind(gvk.ListGVK)

			folder := fmt.Sprintf("namespaces/%s/oadp.openshift.io/dataprotectionapplications", namespace)
			file := folder + "/dataprotectionapplications.yaml"
			for _, dataProtectionApplication := range dataProtectionApplications {
				dataProtectionApplication.GetObjectKind().SetGroupVersionKind(gvk.DataProtectionApplicationGVK)
				list.Items = append(list.Items, runtime.RawExtension{Object: &dataProtectionApplication})

				unsupportedOverridesText := "false"
				if dataProtectionApplication.Spec.UnsupportedOverrides != nil {
					summaryTemplateReplaces["ERRORS"] += fmt.Sprintf(
						"⚠️ DataProtectionApplication **%v** in **%v** namespace is using **unsupportedOverrides**\n\n",
						dataProtectionApplication.Name, namespace,
					)
					unsupportedOverridesText = "⚠️ true"
				}

				dpaStatus := ""
				if len(dataProtectionApplication.Status.Conditions) == 0 {
					dpaStatus = "⚠️ no status"
					summaryTemplateReplaces["ERRORS"] += fmt.Sprintf(
						"⚠️ DataProtectionApplication **%v** with **no status** in **%v** namespace\n\n",
						dataProtectionApplication.Name, namespace,
					)
				} else {
					condition := dataProtectionApplication.Status.Conditions[0]
					if condition.Status == v1.ConditionTrue {
						dpaStatus = fmt.Sprintf("✅ status %s: %s", condition.Type, condition.Status)
					} else {
						dpaStatus = fmt.Sprintf("❌ status %s: %s", condition.Type, condition.Status)
						summaryTemplateReplaces["ERRORS"] += fmt.Sprintf(
							"❌ DataProtectionApplication **%v** with **status %s: %s** in **%v** namespace\n\n",
							dataProtectionApplication.Name, condition.Type, condition.Status, namespace,
						)
					}
				}

				link := fmt.Sprintf("[`yaml`](%s)", file)
				summaryTemplateReplaces["DATA_PROTECTION_APPLICATIONS"] += fmt.Sprintf(
					"| %v | %v | %v | %v | %s |\n",
					namespace, dataProtectionApplication.Name, unsupportedOverridesText, dpaStatus, link,
				)
			}

			// TODO permission
			// TODO need defer somewhere?
			err := os.MkdirAll(path+folder, 0777)
			if err != nil {
				fmt.Printf("An error happened while creating folder structure: %v\n", err)
				// TODO!!!!
				continue
			}
			createYAML(path, file, list)
		}
	} else {
		summaryTemplateReplaces["DATA_PROTECTION_APPLICATIONS"] = "❌ No DataProtectionApplication was found in the cluster"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No DataProtectionApplication was found in the cluster\n\n"
	}
}

func ReplaceBackupStorageLocationsSection(path string, backupStorageLocationList *velerov1.BackupStorageLocationList) {
	if backupStorageLocationList != nil {
		backupStorageLocationsByNamespace := map[string][]velerov1.BackupStorageLocation{}

		for _, backupStorageLocation := range backupStorageLocationList.Items {
			backupStorageLocationsByNamespace[backupStorageLocation.Namespace] = append(backupStorageLocationsByNamespace[backupStorageLocation.Namespace], backupStorageLocation)
		}

		summaryTemplateReplaces["BACKUP_STORAGE_LOCATIONS"] += "| Namespace | Name | spec.default | status.phase | details |\n| --- | --- | --- | --- | --- |\n"
		for namespace, backupStorageLocations := range backupStorageLocationsByNamespace {
			list := &corev1.List{}
			list.GetObjectKind().SetGroupVersionKind(gvk.ListGVK)

			folder := fmt.Sprintf("namespaces/%s/velero.io/backupstoragelocations", namespace)
			file := folder + "/backupstoragelocations.yaml"
			for _, backupStorageLocation := range backupStorageLocations {
				backupStorageLocation.GetObjectKind().SetGroupVersionKind(gvk.BackupStorageLocationGVK)
				list.Items = append(list.Items, runtime.RawExtension{Object: &backupStorageLocation})

				bslStatus := ""
				bslStatusPhase := backupStorageLocation.Status.Phase
				if len(bslStatusPhase) == 0 {
					bslStatus = "⚠️ no status phase"
					summaryTemplateReplaces["ERRORS"] += fmt.Sprintf(
						"⚠️ BackupStorageLocation **%v** with **no status phase** in **%v** namespace\n\n",
						backupStorageLocation.Name, namespace,
					)
				} else {
					if bslStatusPhase == velerov1.BackupStorageLocationPhaseAvailable {
						bslStatus = fmt.Sprintf("✅ status phase %s", bslStatusPhase)
					} else {
						bslStatus = fmt.Sprintf("❌ status phase %s", bslStatusPhase)
						summaryTemplateReplaces["ERRORS"] += fmt.Sprintf(
							"❌ BackupStorageLocation **%v** with **status phase %s** in **%v** namespace\n\n",
							backupStorageLocation.Name, bslStatusPhase, namespace,
						)
					}
				}

				link := fmt.Sprintf("[`yaml`](%s)", file)
				summaryTemplateReplaces["BACKUP_STORAGE_LOCATIONS"] += fmt.Sprintf(
					"| %v | %v | %t | %v | %s |\n",
					namespace, backupStorageLocation.Name, backupStorageLocation.Spec.Default, bslStatus, link,
				)
				// velero get backup-locations
				// NAME              PROVIDER   BUCKET/PREFIX           PHASE         LAST VALIDATED                  ACCESS MODE   DEFAULT
				// velero-sample-1   aws        my-bucket-name/velero   Unavailable   2024-10-21 17:27:45 +0000 UTC   ReadWrite     true

				// oc get bsl -n openshift-adp
				// NAME              PHASE         LAST VALIDATED   AGE    DEFAULT
				// velero-sample-1   Unavailable   22s              112s   true
			}

			// TODO permission
			// TODO need defer somewhere?
			err := os.MkdirAll(path+folder, 0777)
			if err != nil {
				fmt.Printf("An error happened while creating folder structure: %v\n", err)
				// TODO!!!!
				continue
			}
			createYAML(path, file, list)
		}
	} else {
		summaryTemplateReplaces["BACKUP_STORAGE_LOCATIONS"] = "❌ No BackupStorageLocation was found in the cluster"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No BackupStorageLocation was found in the cluster\n\n"
	}
}

func ReplaceVolumeSnapshotLocationsSection(path string, volumeSnapshotLocationList *velerov1.VolumeSnapshotLocationList) {
	if volumeSnapshotLocationList != nil {
		volumeSnapshotLocationsByNamespace := map[string][]velerov1.VolumeSnapshotLocation{}

		for _, volumeSnapshotLocation := range volumeSnapshotLocationList.Items {
			volumeSnapshotLocationsByNamespace[volumeSnapshotLocation.Namespace] = append(volumeSnapshotLocationsByNamespace[volumeSnapshotLocation.Namespace], volumeSnapshotLocation)
		}

		summaryTemplateReplaces["VOLUME_SNAPSHOT_LOCATIONS"] += "| Namespace | Name | details |\n| --- | --- | --- |\n"
		for namespace, volumeSnapshotLocations := range volumeSnapshotLocationsByNamespace {
			list := &corev1.List{}
			list.GetObjectKind().SetGroupVersionKind(gvk.ListGVK)

			folder := fmt.Sprintf("namespaces/%s/velero.io/volumesnapshotlocations", namespace)
			file := folder + "/volumesnapshotlocations.yaml"
			for _, volumeSnapshotLocation := range volumeSnapshotLocations {
				volumeSnapshotLocation.GetObjectKind().SetGroupVersionKind(gvk.VolumeSnapshotLocationGVK)
				list.Items = append(list.Items, runtime.RawExtension{Object: &volumeSnapshotLocation})

				link := fmt.Sprintf("[`yaml`](%s)", file)
				summaryTemplateReplaces["VOLUME_SNAPSHOT_LOCATIONS"] += fmt.Sprintf(
					"| %v | %v | %s |\n",
					namespace, volumeSnapshotLocation.Name, link,
				)
			}

			// TODO permission
			// TODO need defer somewhere?
			err := os.MkdirAll(path+folder, 0777)
			if err != nil {
				fmt.Printf("An error happened while creating folder structure: %v\n", err)
				// TODO!!!!
				continue
			}
			createYAML(path, file, list)
		}
	} else {
		summaryTemplateReplaces["VOLUME_SNAPSHOT_LOCATIONS"] = "❌ No VolumeSnapshotLocation was found in the cluster"
	}
}

// TODO this function writes summary and cluster files
// break into 2
func ReplaceAvailableStorageClassesSection(path string, storageClassList *storagev1.StorageClassList) {
	if storageClassList != nil {
		list := &corev1.List{}
		list.GetObjectKind().SetGroupVersionKind(gvk.ListGVK)

		for _, storageClass := range storageClassList.Items {
			storageClass.GetObjectKind().SetGroupVersionKind(gvk.StorageClassGVK)
			list.Items = append(list.Items, runtime.RawExtension{Object: &storageClass})
		}
		// todo could not create generic function, type/interface/pointer error
		// createYAMLList(storageClassList, gvk.StorageClassGVK)
		summaryTemplateReplaces["STORAGE_CLASSES"] = createYAML(path, "cluster-scoped-resources/storage.k8s.io/storageclasses/storageclasses.yaml", list)
	} else {
		summaryTemplateReplaces["STORAGE_CLASSES"] = "❌ No StorageClass was found in the cluster"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No StorageClass was found in the cluster\n\n"
	}
}

func ReplaceCustomResourceDefinitionsSection(path string, clusterConfig *rest.Config) {
	// TODO error!!!
	client, _ := apiextensionsclientset.NewForConfig(clusterConfig)

	crdsPath := "cluster-scoped-resources/apiextensions.k8s.io/customresourcedefinitions"

	crd, _ := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "dataprotectionapplications.oadp.openshift.io", v1.GetOptions{})
	crd.GetObjectKind().SetGroupVersionKind(gvk.CustomResourceDefinitionGVK)
	// TODO check error
	createYAML(path, crdsPath+"/dataprotectionapplications.yaml", crd)

	crd, _ = client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "clusterserviceversions.operators.coreos.com", v1.GetOptions{})
	crd.GetObjectKind().SetGroupVersionKind(gvk.CustomResourceDefinitionGVK)
	// TODO check error
	createYAML(path, crdsPath+"/clusterserviceversions.yaml", crd)

	summaryTemplateReplaces["CUSTOM_RESOURCE_DEFINITION"] = fmt.Sprintf("For more information, check [`%s`](%s)\n\n", crdsPath, crdsPath)
}

// TODO move to another folder?
func createYAML(path string, yamlPath string, obj runtime.Object) string {
	objFilePath := path + yamlPath
	result := ""
	newFile, err := os.Create(objFilePath)
	if err != nil {
		fmt.Println(err)
		result = "❌ Unable to create " + objFilePath
	} else {
		printer := printers.YAMLPrinter{}
		err = printer.PrintObj(obj, newFile)
		if err != nil {
			fmt.Println(err)
			result = "❌ Unable to write " + objFilePath
		} else {
			result = fmt.Sprintf("For more information, check [`%s`](%s)\n\n", yamlPath, yamlPath)
		}
	}
	defer newFile.Close()
	return result
}

func Write(path string) error {
	if len(summaryTemplateReplaces["ERRORS"]) == 0 {
		summaryTemplateReplaces["ERRORS"] += "No errors happened or were found while running OADP must-gather\n\n"
	}

	summary := summaryTemplate
	for _, key := range summaryTemplateReplacesKeys {
		value, ok := summaryTemplateReplaces[key]
		if !ok {
			return fmt.Errorf("key '%s' not set in SummaryTemplateReplaces", key)
		}
		if len(value) == 0 {
			return fmt.Errorf("value for key '%s' not set in SummaryTemplateReplaces", key)
		}
		summary = strings.ReplaceAll(
			summary,
			fmt.Sprintf("<<%s>>", key),
			value,
		)
	}

	summaryPath := path + "oadp-must-gather-summary.md"
	// TODO permission
	// TODO need defer somewhere?
	err := os.WriteFile(summaryPath, []byte(summary), 0644)
	if err != nil {
		return err
	}

	return nil
}
