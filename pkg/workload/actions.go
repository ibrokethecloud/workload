package workload

import (
	"fmt"
	"os"
	"strings"

	"encoding/json"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const configMapName = "snap-backup"

func (wcc WorkloadCommandConfig) processCommand(args []string) {
	if !wcc.AllKinds {
		if len(args) != 0 {
			switch args[0] {
			case "deploy", "deployment":
				wcc.processDeployments(args[1:])
			case "sts", "statefulset":
				wcc.processStatefulSets(args[1:])
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
	deploymentObjects := wcc.getAllDeployments()
	stsObjects := wcc.getAllStatefulSets()

	if wcc.Stop {
		if len(deploymentObjects) != 0 {
			wcc.stopDeployment(deploymentObjects)
		}
		if len(stsObjects) != 0 {
			wcc.stopStatefulSet(stsObjects)
		}
	} else if wcc.Start {
		if len(deploymentObjects) != 0 {
			wcc.restoreDeployment(deploymentObjects)
		}
		if len(stsObjects) != 0 {
			wcc.restoreStatefulSet(stsObjects)
		}
	} else {
		fmt.Println("A valid --stop/ --start action needs to be specified with the --all-kinds flag")
		os.Exit(1)
	}
}

func (wcc WorkloadCommandConfig) getAllDeployments() (objectList []WorkloadConfig) {
	deploymentList, err := wcc.Client.AppsV1().Deployments(wcc.Namespace).List(wcc.Context, metav1.ListOptions{})
	if err != nil {
		fmt.Println("Error listing deployments %v\n", err)
	}
	for _, deployment := range deploymentList.Items {
		object := WorkloadConfig{
			Name:  deployment.ObjectMeta.Name,
			Scale: *deployment.Spec.Replicas,
			Kind:  "deployment",
		}
		objectList = append(objectList, object)
	}
	return objectList
}

func (wcc WorkloadCommandConfig) getAllStatefulSets() (objectList []WorkloadConfig) {
	stsList, err := wcc.Client.AppsV1().StatefulSets(wcc.Namespace).List(wcc.Context, metav1.ListOptions{})
	if err != nil {
		fmt.Println("Error listing deployments %v\n", err)
	}
	for _, sts := range stsList.Items {
		object := WorkloadConfig{
			Name:  sts.ObjectMeta.Name,
			Scale: *sts.Spec.Replicas,
			Kind:  "statefulset",
		}
		objectList = append(objectList, object)
	}
	return objectList
}

func (wcc WorkloadCommandConfig) processStatefulSets(args []string) {
	errArr := []error{}
	objectList := []WorkloadConfig{}
	if len(args) == 0 {
		fmt.Println("No statefulset name specified. Nothing to do")
		os.Exit(1)
	}

	for _, sts := range args {
		stsObject, err := wcc.Client.AppsV1().StatefulSets(wcc.Namespace).Get(wcc.Context, sts, metav1.GetOptions{})
		if err != nil {
			errArr = append(errArr, err)
		} else {
			fetchedObject := WorkloadConfig{
				Name:  stsObject.ObjectMeta.Name,
				Scale: *stsObject.Spec.Replicas,
				Kind:  "statefulset",
			}
			objectList = append(objectList, fetchedObject)
		}
	}

	if len(errArr) != 0 {
		fmt.Printf("Error processing the request: %v \n", errArr)
	}
	if !wcc.Stop && !wcc.Start {
		fmt.Println("No snap action specified. Listing current state")
		for _, stsObject := range objectList {
			fmt.Printf("Name: %s Scale: %d Kind: %s \n", stsObject.Name, stsObject.Scale, stsObject.Kind)
		}
	} else if wcc.Stop {
		wcc.stopStatefulSet(objectList)
	} else if wcc.Start {
		wcc.restoreStatefulSet(objectList)
	} else {
		fmt.Println("Undefined action")
		os.Exit(1)
	}
}

func (wcc WorkloadCommandConfig) processDeployments(args []string) {
	errArr := []error{}
	objectList := []WorkloadConfig{}
	if len(args) == 0 {
		fmt.Println("No deployment name specified. Nothing to do")
		os.Exit(1)
	}

	for _, deployment := range args {
		deployObject, err := wcc.Client.AppsV1().Deployments(wcc.Namespace).Get(wcc.Context, deployment, metav1.GetOptions{})
		if err != nil {
			errArr = append(errArr, err)
		} else {
			fetchedObject := WorkloadConfig{
				Name:  deployObject.ObjectMeta.Name,
				Scale: *deployObject.Spec.Replicas,
				Kind:  "deployment",
			}
			objectList = append(objectList, fetchedObject)
		}
	}

	if len(errArr) != 0 {
		fmt.Printf("Error processing the request: %v \n", errArr)
		os.Exit(1)
	}

	if !wcc.Stop && !wcc.Start {
		fmt.Println("No workload action specified. Listing current state")
		for _, stsObject := range objectList {
			fmt.Printf("Name: %s Scale: %d Kind: %s \n", stsObject.Name, stsObject.Scale, stsObject.Kind)
		}
	} else if wcc.Stop {
		wcc.stopDeployment(objectList)
	} else if wcc.Start {
		wcc.restoreDeployment(objectList)
	} else {
		fmt.Println("Undefined action")
		os.Exit(1)
	}
}

func listNamespaces(wcc *WorkloadCommandConfig) ([]string, error) {
	namespaces := []string{}
	nsList, err := wcc.Client.CoreV1().Namespaces().List(wcc.Context, metav1.ListOptions{})
	if err != nil {
		return namespaces, err
	}

	for _, ns := range nsList.Items {
		namespaces = append(namespaces, ns.ObjectMeta.Name)
	}

	return namespaces, nil
}

func (wcc WorkloadCommandConfig) updateConfigMap(objects []WorkloadConfig) {
	cmData := make(map[string]string)
	backupConfigMap, err := wcc.Client.CoreV1().ConfigMaps(wcc.Namespace).Get(wcc.Context, configMapName, metav1.GetOptions{})
	cmData = backupConfigMap.Data
	if err != nil {
		if strings.Contains(err.Error(), "configmaps \"snap-backup\" not found") {
			wcc.createConfigMap()
		} else {
			fmt.Printf("Error fetching ConfigMap %v", err)
		}
	}
	for _, objectState := range objects {
		fullName := objectState.Kind + "-" + objectState.Name
		jsonData, err := json.Marshal(objectState)
		if err != nil {
			fmt.Printf("Error updating configMap data %v\n", err)
		}
		cmData[fullName] = string(jsonData)
	}

	backupConfigMap.Data = cmData
	_, err = wcc.Client.CoreV1().ConfigMaps(wcc.Namespace).Update(wcc.Context, backupConfigMap, metav1.UpdateOptions{})
	if err != nil {
		fmt.Println("Error updating configMap object %v\n", err)
	}
}

func (wcc WorkloadCommandConfig) createConfigMap() {
	snapBackup := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: configMapName,
		},
	}
	_, err := wcc.Client.CoreV1().ConfigMaps(wcc.Namespace).Create(wcc.Context, snapBackup, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("Error creating backup configmap %v \n", err)
		os.Exit(1)
	}
}

func (wcc WorkloadCommandConfig) fetchSavedScale(object WorkloadConfig) (scale int32, err error) {
	oc := WorkloadConfig{}
	storedState, err := wcc.Client.CoreV1().ConfigMaps(wcc.Namespace).Get(wcc.Context, configMapName, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}

	if state, ok := storedState.Data[object.Kind+"-"+object.Name]; ok {
		err = json.Unmarshal([]byte(state), &oc)
		if err != nil {
			return 0, err
		}
	}

	return oc.Scale, err
}

func (wcc WorkloadCommandConfig) stopDeployment(objectList []WorkloadConfig) {
	wcc.updateConfigMap(objectList)
	scale := int32(0)
	for _, deployment := range objectList {
		wcc.actionDeployment(deployment, &scale)
	}
}

func (wcc WorkloadCommandConfig) restoreDeployment(objectList []WorkloadConfig) {
	for _, object := range objectList {
		savedScale, err := wcc.fetchSavedScale(object)
		if err != nil {
			fmt.Printf("Error fetching saved scale: %v\n", err)
			os.Exit(1)
		}
		wcc.actionDeployment(object, &savedScale)
	}

}

func (wcc WorkloadCommandConfig) actionDeployment(object WorkloadConfig, scale *int32) {
	deployment, err := wcc.Client.AppsV1().Deployments(wcc.Namespace).Get(wcc.Context, object.Name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error fetching deployment %v\n", err)
		os.Exit(1)
	}
	deployment.Spec.Replicas = scale
	_, err = wcc.Client.AppsV1().Deployments(wcc.Namespace).Update(wcc.Context, deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("Error updating the deployment %v\n", err)
		os.Exit(1)
	}
}

func (wcc WorkloadCommandConfig) stopStatefulSet(objectList []WorkloadConfig) {
	wcc.updateConfigMap(objectList)
	scale := int32(0)
	for _, sts := range objectList {
		wcc.actionStatefulSet(sts, &scale)
	}
}

func (wcc WorkloadCommandConfig) restoreStatefulSet(objectList []WorkloadConfig) {
	for _, object := range objectList {
		savedScale, err := wcc.fetchSavedScale(object)
		if err != nil {
			fmt.Printf("Error fetching saved scale: %v\n", err)
			os.Exit(1)
		}
		wcc.actionStatefulSet(object, &savedScale)
	}
}

func (wcc WorkloadCommandConfig) actionStatefulSet(object WorkloadConfig, scale *int32) {
	sts, err := wcc.Client.AppsV1().StatefulSets(wcc.Namespace).Get(wcc.Context, object.Name, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("Error fetching statefulset %v\n", err)
		os.Exit(1)
	}
	sts.Spec.Replicas = scale
	_, err = wcc.Client.AppsV1().StatefulSets(wcc.Namespace).Update(wcc.Context, sts, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("Error updating the statefulset %v\n", err)
		os.Exit(1)
	}
}
