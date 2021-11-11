package main

import (
	"context"
	"encoding/json"
	"fmt"
	v12 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"net/http"
	"time"
)

type Repository struct {
	RepoName  string `json:"repo_name"`
	RepoUrl   string `json:"repo_url"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type PushedData struct {
	Tag      string `json:"tag"`
	PushedAt uint64 `json:"pushed_at"`
}

type DockerHubEvent struct {
	CallbackUrl string     `json:"callback_url"`
	Repository  Repository `json:"repository"`
	PushedData  PushedData `json:"push_data"`
}

var deploymentClient v1.DeploymentInterface

func handleHookRequest(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		var event DockerHubEvent
		if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, deployment := range deployments.Items {
			if isTurbineApp(deployment) && containsContainer(deployment, event.Repository.RepoName) {
				err := restartDeployment(deployment.Name, deploymentClient)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}
		}
	default:
		http.Error(w, "Unsupported HTTP method, this is POST-only", http.StatusBadRequest)
	}
}

func handleKubernetesRequest(w http.ResponseWriter, req *http.Request) {
	deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, deployment := range deployments.Items {
		if isTurbineApp(deployment) {
			err := restartDeployment(deployment.Name, deploymentClient)
			if err != nil {
				panic(err.Error())
			}
		}
	}
	fmt.Printf("There are %d deployments in the cluster\n", len(deployments.Items))
}

func isTurbineApp(deployment v12.Deployment) bool {
	val, ok := deployment.Spec.Template.Annotations["turbine/enabled"]
	return ok && "true" == val
}

func containsContainer(deployment v12.Deployment, containerName string) bool {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Image == containerName {
			return true
		}
	}
	return false
}

func restartDeployment(deploymentName string, deploymentClient v1.DeploymentInterface) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fmt.Printf("Trying to restart deployment %s\n", deploymentName)
		result, err := deploymentClient.Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		result.Spec.Template.Annotations["turbine/restartedAt"] = time.Now().Format(time.RFC3339)
		_, updateErr := deploymentClient.Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})
}

func handleGithubRequest(w http.ResponseWriter, req *http.Request) {
	fmt.Println("I just received a request from Github action")
}

func main() {
	deploymentClient = createDeploymentClient("default")

	http.HandleFunc("/webhook", handleHookRequest)
	http.HandleFunc("/k8s", handleKubernetesRequest)
	http.HandleFunc("/gh-action", handleGithubRequest)
	http.ListenAndServe(":8080", nil)
}

func createDeploymentClient(namespace string) v1.DeploymentInterface {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset.AppsV1().Deployments(namespace)
}
