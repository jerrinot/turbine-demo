package internal

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type DeploymentController struct {
	kubernetesProxy *KubernetesProxy
}

func NewDeploymentController(p *KubernetesProxy) *DeploymentController {
	return &DeploymentController{kubernetesProxy: p}
}

func (dc *DeploymentController) HandleNewDeploymentRequest(w http.ResponseWriter, r *http.Request) {
	var applicationDescriptor turbineService
	if err := json.NewDecoder(r.Body).Decode(&applicationDescriptor); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Request to deploy a new application: %s\n", applicationDescriptor)
	if dc.kubernetesProxy.containsDeployment(r.Context(), applicationDescriptor.Name) {
		http.Error(w, fmt.Sprintf("Application %s already exist", applicationDescriptor.Name), http.StatusConflict)
		return
	}

	err := dc.kubernetesProxy.newDeployment(r.Context(), applicationDescriptor)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if applicationDescriptor.Expose {
		err := dc.kubernetesProxy.exposeService(r.Context(), applicationDescriptor)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (dc *DeploymentController) HandleListDeploymentRequest(w http.ResponseWriter, r *http.Request) {
	turbineApps, err := dc.kubernetesProxy.getAllApps(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	err = json.NewEncoder(w).Encode(turbineApps)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (dc *DeploymentController) HandleDeploymentDeleteRequest(w http.ResponseWriter, r *http.Request) {
	pathParams := mux.Vars(r)
	applicationName := pathParams["application"]
	log.Printf("Handling delete %s request\n", applicationName)
	if err := dc.kubernetesProxy.deleteApplication(r.Context(), applicationName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
