package main

import (
	"fmt"
	"github.com/gorilla/mux"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"log"
	"net/http"
)

type TurbineService struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Port     int32  `json:"port"`
	Replicas int32  `json:"replicas"`
	Expose   bool   `json:"expose"`
	IP       string `json:"ip"`
}

func (c TurbineService) String() string {
	return fmt.Sprintf("Name: %s, Image: %s, Port: %d, Replicas: %d, Expose: %t", c.Name,
		c.Image, c.Port, c.Replicas, c.Expose)
}

var (
	clusterResources *ClientResources
)

func main() {
	clusterResources = createClients("default")
	r := mux.NewRouter()
	r.HandleFunc("/webhook", handleDockerHubHookRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment", handleListDeploymentRequest).Methods(http.MethodGet)
	r.HandleFunc("/deployment", handleNewDeploymentRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment/{application}", handleDeploymentDeleteRequest).Methods(http.MethodDelete)
	log.Fatal(http.ListenAndServe(":8080", r))
}
