package templates

import (
	"fmt"
	"os"
	"slices"
	"strings"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
)

var (
	summaryTemplateReplacesKeys = []string{
		"ERRORS",
		"CLUSTER_ID", "OCP_VERSION", "CLOUD", "ARCH", "OCP_CAPABILITIES",
		"OADP_VERSIONS",
		"STORAGE_CLASSES",
	}
	summaryTemplateReplaces = map[string]string{}
)

// TODO https://stackoverflow.com/a/31742265
// TODO https://github.com/kubernetes-sigs/kubebuilder/blob/master/pkg/plugins/golang/v4/scaffolds/internal/templates/readme.go
const summaryTemplate = `# OADP must-gather summary version ???

## Errors

<<ERRORS>>

## Cluster information

| Cluster ID | OpenShift version | Kubernetes version | Cloud provider | Architecture |
| ---------- | ----------------- | ------------------ | -------------- | ------------ |
| <<CLUSTER_ID>> | <<OCP_VERSION>> | ??? | <<CLOUD>> | <<ARCH>> |

Cluster capabilities
<<OCP_CAPABILITIES>>

## OADP operator installation information

<<OADP_VERSIONS>>

## File system information

TODO

## Velero client version ???

## Velero deployment information in namespace ???

## Available StorageClasses in cluster

<<STORAGE_CLASSES>>

## CSI VolumeSnapshotClasses

TODO

## DataProtectionApplication ???/???

TODO
`

func init() {
	for _, key := range summaryTemplateReplacesKeys {
		summaryTemplateReplaces[key] = ""
	}
}

func ReplaceClusterInformationSection(clusterID string, clusterVersion *openshiftconfigv1.ClusterVersion, infrastructure *openshiftconfigv1.Infrastructure, nodeList *corev1.NodeList) {
	summaryTemplateReplaces["CLUSTER_ID"] = clusterID

	if clusterVersion != nil {
		summaryTemplateReplaces["OCP_VERSION"] = clusterVersion.Status.Desired.Version
		capabilitiesText := ""
		for _, cap := range clusterVersion.Status.Capabilities.KnownCapabilities {
			if slices.Contains(clusterVersion.Status.Capabilities.EnabledCapabilities, cap) {
				capabilitiesText += fmt.Sprintf("\n- ✅ %v", cap)
			} else {
				capabilitiesText += fmt.Sprintf("\n- ❌ %v", cap)
			}
		}
		summaryTemplateReplaces["OCP_CAPABILITIES"] = capabilitiesText
		// here, we could have a list of important capabilities and only check if those are enabled
	} else {
		// this is code is unreachable?
		summaryTemplateReplaces["OCP_VERSION"] = "❌ error"
		summaryTemplateReplaces["OCP_CAPABILITIES"] = "❌ error"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No ClusterVersion found in cluster\n"
	}

	if infrastructure != nil {
		cloudProvider := string(infrastructure.Spec.PlatformSpec.Type)
		summaryTemplateReplaces["CLOUD"] = cloudProvider
	} else {
		summaryTemplateReplaces["CLOUD"] = "❌ error"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No Infrastructure found in cluster\n"
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
		summaryTemplateReplaces["ERRORS"] += "⚠️ No Node found in cluster\n"
	}
	// TODO maybe nil case can be simplified by initializing everything with an error state/message
}

func ReplaceOADPOperatorInstallationSection(clusterServiceVersionList *operatorsv1alpha1.ClusterServiceVersionList) {
	if clusterServiceVersionList != nil {
		oadpOperatorsText := ""
		for _, csv := range clusterServiceVersionList.Items {
			// prod?
			// community?
			// other CSVs that should be important for us?
			// dev? https://github.com/openshift/oadp-operator/blob/5601dcfd0a07468f496ddb70ab570ccff1b4f0cc/bundle/manifests/oadp-operator.clusterserviceversion.yaml#L598
			if csv.Spec.DisplayName == "OADP Operator" {
				oadpOperatorsText += fmt.Sprintf("Found '%v' version '%v' installed in '%v' namespace\n\n", csv.Spec.DisplayName, csv.Spec.Version, csv.Namespace)
			}
		}
		if len(oadpOperatorsText) == 0 {
			summaryTemplateReplaces["OADP_VERSIONS"] = "❌ No OADP Operator was found in the cluster"
			summaryTemplateReplaces["ERRORS"] += "🚫 No OADP Operator was found in the cluster\n"
		} else {
			summaryTemplateReplaces["OADP_VERSIONS"] = oadpOperatorsText
		}
	} else {
		summaryTemplateReplaces["OADP_VERSIONS"] = "❌ No OADP Operator was found in the cluster"
		summaryTemplateReplaces["ERRORS"] += "🚫 No ClusterServiceVersion found in cluster\n"
	}
}

// TODO this function writes summary and cluster files
// break into 2
func ReplaceAvailableStorageClassesSection(path string, storageClassList *storagev1.StorageClassList) {
	if storageClassList != nil {
		list := &corev1.List{}
		listGVK := schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "List",
		}
		list.GetObjectKind().SetGroupVersionKind(listGVK)

		storageClasses := ""
		for _, storageClass := range storageClassList.Items {
			storageClasses += fmt.Sprintf("Found '%v' StorageClass\n\n", storageClass.Name)
			storageClassGVK := schema.GroupVersionKind{
				Group:   "storage.k8s.io",
				Version: "v1",
				Kind:    "StorageClass",
			}
			storageClass.GetObjectKind().SetGroupVersionKind(storageClassGVK)
			list.Items = append(list.Items, runtime.RawExtension{Object: &storageClass})
			// annotations
			//   storageclass.kubernetes.io/is-default-class
			//   storageclass.kubevirt.io/is-default-virt-class
			// storageClass.Provisioner
		}
		storageClassesFilePath := path + "/cluster-scoped-resources/storage.k8s.io/storageclasses.yaml"
		newFile, err := os.Create(storageClassesFilePath)
		if err != nil {
			fmt.Println(err)
			storageClasses += "❌ Unable to create " + storageClassesFilePath
		} else {
			printer := printers.YAMLPrinter{}
			err = printer.PrintObj(list, newFile)
			if err != nil {
				fmt.Println(err)
				storageClasses += "❌ Unable to write " + storageClassesFilePath
			} else {
				storageClasses += "For more information, check [`cluster-scoped-resources/storage.k8s.io/storageclasses.yaml`](cluster-scoped-resources/storage.k8s.io/storageclasses.yaml)\n"
			}
		}
		defer newFile.Close()
		summaryTemplateReplaces["STORAGE_CLASSES"] = storageClasses
	} else {
		summaryTemplateReplaces["STORAGE_CLASSES"] = "❌ No StorageClass was found in the cluster"
		summaryTemplateReplaces["ERRORS"] += "⚠️ No StorageClass was found in the cluster\n"
	}
}

func Write(path string) error {
	if len(summaryTemplateReplaces["ERRORS"]) == 0 {
		summaryTemplateReplaces["ERRORS"] += "No errors happened or were found while running OADP must-gather\n"
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

	summaryPath := path + "/oadp-must-gather-summary.md"
	// TODO permission
	// TODO need defer somewhere?
	err := os.WriteFile(summaryPath, []byte(summary), 0644)
	if err != nil {
		return err
	}

	return nil
}
