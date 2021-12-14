package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strconv"
)

func handleNewDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	var applicationDescriptor TurbineService
	//todo: validate the descriptor

	if err := json.NewDecoder(req.Body).Decode(&applicationDescriptor); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Printf("Request to deploy a new application: %s\n", applicationDescriptor)
	_, err := clusterResources.DeploymentClient.Get(context.TODO(), applicationDescriptor.Name, metav1.GetOptions{})
	if err == nil {
		http.Error(w, fmt.Sprintf("Application %s already exist", applicationDescriptor.Name), http.StatusConflict)
		return
	}
	deployment := constructDeploymentDescriptor(applicationDescriptor)
	deployment, err = clusterResources.DeploymentClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if applicationDescriptor.Expose {
		service := constructServiceDescriptor(applicationDescriptor)
		_, err := clusterResources.ServiceClient.Create(context.TODO(), service, metav1.CreateOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func handleListDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	deployments, err := clusterResources.DeploymentClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var allTurbineApps []TurbineService
	for _, deployment := range deployments.Items {
		if isTurbineApp(deployment) {
			port, err := strconv.Atoi(readAnnotation(deployment, "turbine/port", "0"))
			if err != nil {
				port = 0
			}
			exposed, err := strconv.ParseBool(readAnnotation(deployment, "turbine/exposed", "false"))
			if err != nil {
				exposed = false
			}
			var ip = ""
			if exposed {
				service, _ := clusterResources.ServiceClient.Get(context.TODO(), deployment.Name, metav1.GetOptions{})
				if service != nil {
					ingress := service.Status.LoadBalancer.Ingress
					if len(ingress) != 0 {
						ip = ingress[0].IP
					}
				}
			}
			turbineApp := TurbineService{
				Name:     deployment.Name,
				Image:    deployment.Spec.Template.Spec.Containers[0].Image,
				Port:     int32(port),
				Replicas: *deployment.Spec.Replicas,
				Expose:   exposed,
				IP:       ip,
			}
			allTurbineApps = append(allTurbineApps, turbineApp)
		}
	}
	err = json.NewEncoder(w).Encode(allTurbineApps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func handleDeploymentDeleteRequest(w http.ResponseWriter, req *http.Request) {
	pathParams := mux.Vars(req)
	applicationName := pathParams["application"]
	fmt.Printf("Handling delete %s request\n", applicationName)

	deployment, err := clusterResources.DeploymentClient.Get(context.TODO(), applicationName, metav1.GetOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Application %s not found", applicationName), http.StatusNotFound)
		return
	}
	if !isTurbineApp(*deployment) {
		http.Error(w, fmt.Sprintf("Application %s is not a Turbine-application", applicationName), http.StatusBadRequest)
		return
	}

	serviceShouldExist := readAnnotation(*deployment, "turbine/exposed", "false")
	if clusterResources.DeploymentClient.Delete(context.TODO(), applicationName, metav1.DeleteOptions{}) != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if serviceShouldExist == "true" {
		if clusterResources.ServiceClient.Delete(context.TODO(), applicationName, metav1.DeleteOptions{}) != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
