package internal

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

type DeploymentController struct {
	kubernetesProxy *KubernetesProxy
}

func NewDeploymentController(proxy *KubernetesProxy) *DeploymentController {
	return &DeploymentController{kubernetesProxy: proxy}
}

func (deployments *DeploymentController) HandleNewDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	var applicationDescriptor turbineService
	if err := json.NewDecoder(req.Body).Decode(&applicationDescriptor); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Printf("Request to deploy a new application: %s\n", applicationDescriptor)
	if deployments.kubernetesProxy.containsDeployment(req.Context(), applicationDescriptor.Name) {
		http.Error(w, fmt.Sprintf("Application %s already exist", applicationDescriptor.Name), http.StatusConflict)
		return
	}

	err := deployments.kubernetesProxy.newDeployment(req.Context(), applicationDescriptor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if applicationDescriptor.Expose {
		err := deployments.kubernetesProxy.exposeService(req.Context(), applicationDescriptor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (deployments *DeploymentController) HandleListDeploymentRequest(w http.ResponseWriter, req *http.Request) {
	turbineApps, err := deployments.kubernetesProxy.getAllApps(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	err = json.NewEncoder(w).Encode(turbineApps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (deployments *DeploymentController) HandleDeploymentDeleteRequest(w http.ResponseWriter, req *http.Request) {
	pathParams := mux.Vars(req)
	applicationName := pathParams["application"]
	fmt.Printf("Handling delete %s request\n", applicationName)
	if err := deployments.kubernetesProxy.deleteApplication(req.Context(), applicationName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
