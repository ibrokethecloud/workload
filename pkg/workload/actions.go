package workload

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configMapName      = "snap-backup"
	snapResource       = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	deploymentResource = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	stsResource        = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
)

func (wcc WorkloadCommandConfig) processCommand(args []string) {
	if !wcc.AllKinds {
		if len(args) != 0 {
			switch args[0] {
			case "deploy", "deployment":
				wcc.processWorkload(deploymentResource, args[1:])
			case "sts", "statefulset":
				wcc.processWorkload(stsResource, args[1:])
			default:
				fmt.Println("Supported kinds are deployment and statefulset")
				os.Exit(1)
			}
		} else {
			fmt.Println("No argument provideded and --all-kinds is not set. Nothing to do.")
			os.Exit(1)
		}
	} else {
		wcc.processAllKinds()
	}

}

func (wcc WorkloadCommandConfig) processAllKinds() {
	objectList := []WorkloadConfig{}
	objectList = append(objectList, wcc.getAllWorkloads(deploymentResource)...)
	objectList = append(objectList, wcc.getAllWorkloads(stsResource)...)

	if wcc.Stop {
		if len(objectList) != 0 {
			wcc.stopWorkload(objectList)
		}
	} else if wcc.Start {
		if len(objectList) != 0 {
			wcc.restoreWorkload(objectList)
		}
	} else {
		fmt.Println("A valid --stop/ --start action needs to be specified with the --all-kinds flag")
		os.Exit(1)
	}
}

func (wcc WorkloadCommandConfig) getAllWorkloads(resourceType schema.GroupVersionResource) (objectList []WorkloadConfig) {

	workloadList, err := wcc.Client.Resource(resourceType).Namespace(wcc.Namespace).
		List(wcc.Context, metav1.ListOptions{})
	if err != nil {
		fmt.Errorf("Error listing resources in the namespace %v\n", err)
		os.Exit(1)
	}

	for _, w := range workloadList.Items {
		object, errArr := generateWorkloadConfig(&w)
		if len(errArr) != 0 {
			fmt.Errorf("Error generating Workload object %v\n", errArr)
			os.Exit(1)
		}
		objectList = append(objectList, object)
	}

	return objectList
}

func generateWorkloadConfig(object *unstructured.Unstructured) (w WorkloadConfig, errArr []error) {
	name, ok, err := unstructured.NestedString(object.Object, "metadata", "name")
	if !ok || err != nil {
		errArr = append(errArr, err)
	}
	scale, ok, err := unstructured.NestedInt64(object.Object, "spec", "replicas")
	if !ok || err != nil {
		errArr = append(errArr, err)
	}
	kind, ok, err := unstructured.NestedString(object.Object, "kind")
	if !ok || err != nil {
		errArr = append(errArr, err)
	}
	w.Name = strings.ToLower(name)
	w.Scale = scale
	w.Kind = strings.ToLower(kind)
	return w, errArr
}

func (wcc WorkloadCommandConfig) processWorkload(resource schema.GroupVersionResource,
	args []string) (objectList []WorkloadConfig) {
	if len(args) == 0 {
		fmt.Printf("No %v name specified. Nothing to do \n", resource.Resource)
		os.Exit(1)

	}

	for _, workload := range args {
		workloadObject, err := wcc.Client.Resource(resource).Namespace(wcc.Namespace).
			Get(wcc.Context, workload, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Error fetching workload %s %v\n", workload, err)
			os.Exit(1)
		}
		object, errArr := generateWorkloadConfig(workloadObject)
		if len(errArr) != 0 {
			fmt.Printf("Error processing workload response into WorkloadConfig %v\n", errArr)
			os.Exit(1)
		}
		objectList = append(objectList, object)

	}

	if !wcc.Stop && !wcc.Start {
		fmt.Println("No snap action specified. Listing current state")
		for _, stsObject := range objectList {
			fmt.Printf("Name: %s Scale: %d Kind: %s \n", stsObject.Name, stsObject.Scale, stsObject.Kind)
		}
	} else if wcc.Stop {
		wcc.stopWorkload(objectList)
	} else if wcc.Start {
		wcc.restoreWorkload(objectList)
	} else {
		fmt.Errorf("Undefined action \n")
		os.Exit(1)
	}

	return objectList
}
func (wcc WorkloadCommandConfig) updateConfigMap(objects []WorkloadConfig) {
	cmData := make(map[string]string)
	snapBackup, err := wcc.Client.Resource(snapResource).Namespace(wcc.Namespace).
		Get(wcc.Context, configMapName, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "configmaps \"snap-backup\" not found") {
			snapBackup = wcc.createConfigMap()
		} else {
			fmt.Printf("Error fetching ConfigMap %v", err)
			os.Exit(1)
		}
	}

	tmpData, ok, _ := unstructured.NestedStringMap(snapBackup.Object, "data")
	if ok {
		cmData = tmpData
	}

	for _, objectState := range objects {
		fullName := objectState.Kind + "-" + objectState.Name
		jsonData, err := json.Marshal(objectState)
		if err != nil {
			fmt.Printf("Error marshalling configMap data %v\n", err)
		}
		cmData[fullName] = string(jsonData)
	}

	if err := unstructured.SetNestedStringMap(snapBackup.Object, cmData, "data"); err != nil {
		fmt.Printf("Unable to update ConfigMap data\n", err)
		os.Exit(1)
	}

	if _, err := wcc.Client.Resource(snapResource).Namespace(wcc.Namespace).
		Update(wcc.Context, snapBackup, metav1.UpdateOptions{}); err != nil {
		fmt.Printf("Unable to updated ConfigMap object %v\n", err)
		os.Exit(1)
	}

}

func (wcc WorkloadCommandConfig) createConfigMap() (snapBackupObject *unstructured.Unstructured) {
	snapBackup := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": configMapName,
			},
		},
	}

	snapBackupObject, err := wcc.Client.Resource(snapResource).Namespace(wcc.Namespace).
		Create(wcc.Context, snapBackup, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("Unable to create configmap %v\n", configMapName)
		os.Exit(1)
	}
	return snapBackupObject
}

func (wcc WorkloadCommandConfig) fetchSavedScale(object WorkloadConfig) (scale int64, err error) {
	configMapNested, err := wcc.fetchSavedMap()
	if err != nil {
		fmt.Printf("Error fetching saved config map: %v\n")
		os.Exit(1)
	}

	result, ok := configMapNested[object.Kind+"-"+object.Name]

	oc := WorkloadConfig{}
	if ok {
		err = json.Unmarshal([]byte(result), &oc)
	} else {
		fmt.Printf("Object %v not stored in snap-backup. Aborting now..\n", object.Name)
		os.Exit(1)
	}

	return oc.Scale, err
}

func (wcc WorkloadCommandConfig) fetchSavedMap() (configMapNested map[string]string, err error) {
	snapBackup, err := wcc.Client.Resource(snapResource).Namespace(wcc.Namespace).
		Get(wcc.Context, configMapName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Unable to fetch %v from cluster \n", configMapName)
		os.Exit(1)
	}

	configMapNested, _, err = unstructured.NestedStringMap(snapBackup.Object, "data")

	return configMapNested, err
}

func (wcc WorkloadCommandConfig) actionWorkload(object WorkloadConfig, scale int64) {
	workload := schema.GroupVersionResource{}
	if object.Kind == "deployment" {
		workload = deploymentResource
	} else if object.Kind == "statefulset" {
		workload = stsResource
	} else {
		fmt.Errorf("Unknown resource type to action\n")
		os.Exit(1)
	}

	workloadObject, err := wcc.Client.Resource(workload).Namespace(wcc.Namespace).
		Get(wcc.Context, object.Name, metav1.GetOptions{})
	if err != nil {
		fmt.Errorf("Error fetching workload %v\n", err)
		os.Exit(1)
	}

	if err := unstructured.SetNestedField(workloadObject.Object, scale, "spec", "replicas"); err != nil {
		fmt.Errorf("Unable to change workload scale %s %s %v\n", object.Kind, object.Name, err)
		os.Exit(1)
	}
	_, err = wcc.Client.Resource(workload).Namespace(wcc.Namespace).Update(wcc.Context, workloadObject, metav1.UpdateOptions{})
	if err != nil {
		fmt.Errorf("Error updating the workload scale %v\n", err)
		os.Exit(1)
	}
}

// stop workload
func (wcc WorkloadCommandConfig) stopWorkload(objectList []WorkloadConfig) {
	wcc.updateConfigMap(objectList)
	scale := int64(0)
	for _, workload := range objectList {
		wcc.actionWorkload(workload, scale)
	}
}

// start workload
func (wcc WorkloadCommandConfig) restoreWorkload(objectList []WorkloadConfig) {
	for _, object := range objectList {
		savedScale, err := wcc.fetchSavedScale(object)
		if err != nil {
			fmt.Printf("Error fetching saved scale: %v\n", err)
			os.Exit(1)
		}
		wcc.actionWorkload(object, savedScale)
	}
}

// Update workloads
func (wcc WorkloadCommandConfig) updateWorkloadObject(objectWorkloadConfig WorkloadConfig, scale int64) {
	workloadResource := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: objectWorkloadConfig.Kind}

	workloadResult, err := wcc.Client.Resource(workloadResource).Namespace(wcc.Namespace).
		Get(wcc.Context, objectWorkloadConfig.Name, metav1.GetOptions{})

	if err != nil {
		fmt.Errorf("Unable to fetch %s %s :%v\n",
			objectWorkloadConfig.Kind, objectWorkloadConfig.Name, err)
		os.Exit(1)
	}

	if err := unstructured.SetNestedField(workloadResult.Object, scale, "spec", "replicas"); err != nil {
		fmt.Errorf("Unable to update workload %s %s %v\n",
			objectWorkloadConfig.Kind, objectWorkloadConfig.Name, err)
		os.Exit(1)
	}
}
