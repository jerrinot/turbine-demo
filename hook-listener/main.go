package main

import (
	"context"
	"encoding/json"
	"fmt"
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

func handleHookRequest(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "POST":
		var event DockerHubEvent
		if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Println(event)
	default:
		http.Error(w, "Unsupported HTTP method, this is POST-only", http.StatusBadRequest)
	}
}

func handleKubernetesRequest(w http.ResponseWriter, req *http.Request) {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())

	}
	deploymentClient := clientset.AppsV1().Deployments("default")
	deployments, err := deploymentClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, deployment := range deployments.Items {
		if val, ok := deployment.Spec.Template.Annotations["turbine/enabled"]; ok {
			fmt.Printf("Deployment %s has annotation turbine/enabled set to %s\n", deployment.Name, val)
			if "true" == val {
				restartDeployment(deployment.Name, deploymentClient)
			}
		}
	}

	fmt.Printf("There are %d deployments in the cluster\n", len(deployments.Items))
}

func restartDeployment(deploymentName string, deploymentClient v1.DeploymentInterface) {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		fmt.Printf("Trying to restart deployment %s\n", deploymentName)
		var result, getErr = deploymentClient.Get(context.TODO(), deploymentName, metav1.GetOptions{})
		if getErr != nil {
			panic(fmt.Errorf("Failed to get latest version of Deployment: %v", getErr))
		}

		result.Spec.Template.Annotations["turbine/restartedAt"] = time.Now().Format(time.RFC3339)
		_, updateErr := deploymentClient.Update(context.TODO(), result, metav1.UpdateOptions{})
		return updateErr
	})
	if retryErr != nil {
		panic(fmt.Errorf("Update failed: %v", retryErr))
	}
}

func handleGithubRequest(w http.ResponseWriter, req *http.Request) {
	fmt.Println("I just received a request from Github action")
}

func main() {
	http.HandleFunc("/webhook", handleHookRequest)
	http.HandleFunc("/k8s", handleKubernetesRequest)
	http.HandleFunc("/gh-action", handleGithubRequest)
	http.ListenAndServe(":8080", nil)
}
