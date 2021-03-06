package internal

import (
	"encoding/json"
	"net/http"
)

type dockerHubEvent struct {
	CallbackUrl string     `json:"callback_url"`
	Repository  repository `json:"repository"`
	PushedData  pushedData `json:"push_data"`
}

type repository struct {
	RepoName  string `json:"repo_name"`
	RepoUrl   string `json:"repo_url"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type pushedData struct {
	Tag      string `json:"tag"`
	PushedAt uint64 `json:"pushed_at"`
}

type DockerHubController struct {
	proxy *KubernetesProxy
}

func NewDockerHubController(p *KubernetesProxy) *DockerHubController {
	return &DockerHubController{proxy: p}
}

func (handler DockerHubController) HandleDockerHubHookRequest(w http.ResponseWriter, r *http.Request) {
	var event dockerHubEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	deployments, err := handler.proxy.listDeployment(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, deployment := range deployments.Items {
		if isTurbineApp(deployment) && containsContainer(deployment, event.Repository.RepoName, event.PushedData.Tag) {
			err := handler.proxy.restartDeployment(r.Context(), deployment.Name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
}
