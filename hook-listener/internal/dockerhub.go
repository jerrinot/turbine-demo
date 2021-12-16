package internal

import (
	"context"
	"encoding/json"
	"net/http"
)

type DockerHubEvent struct {
	CallbackUrl string     `json:"callback_url"`
	Repository  Repository `json:"repository"`
	PushedData  PushedData `json:"push_data"`
}

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

func HandleDockerHubHookRequest(w http.ResponseWriter, req *http.Request) {
	var event DockerHubEvent
	if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	deployments, err := listDeployment(context.TODO())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, deployment := range deployments.Items {
		if isTurbineApp(deployment) && containsContainer(deployment, event.Repository.RepoName, event.PushedData.Tag) {
			err := restartDeployment(deployment.Name, clusterResources.DeploymentClient)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
}
