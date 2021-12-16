package internal

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

type Deployments struct {
	kubernetesController *KubernetesController
}

func NewDeployment(controller *KubernetesController) *Deployments {
	return &Deployments{kubernetesController: controller}
}

func (deployments *Deployments) HandleNewDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	var applicationDescriptor turbineService
	if err := json.NewDecoder(req.Body).Decode(&applicationDescriptor); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Printf("Request to deploy a new application: %s\n", applicationDescriptor)
	if deployments.kubernetesController.containsDeployment(applicationDescriptor.Name) {
		http.Error(w, fmt.Sprintf("Application %s already exist", applicationDescriptor.Name), http.StatusConflict)
		return
	}

	err := deployments.kubernetesController.newDeployment(applicationDescriptor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if applicationDescriptor.Expose {
		err := deployments.kubernetesController.exposeService(applicationDescriptor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (deployments *Deployments) HandleListDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	turbineApps, err := deployments.kubernetesController.getAllApps()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	err = json.NewEncoder(w).Encode(turbineApps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (deployments *Deployments) HandleDeploymentDeleteRequest(w http.ResponseWriter, req *http.Request) {
	pathParams := mux.Vars(req)
	applicationName := pathParams["application"]
	fmt.Printf("Handling delete %s request\n", applicationName)
	if err := deployments.kubernetesController.deleteApplication(applicationName); err != nil {
		w.Header()
	}
	w.WriteHeader(http.StatusOK)
}
