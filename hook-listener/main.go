package main

import (
	"fmt"
	"github.com/gorilla/mux"
	appsv1 "k8s.io/api/apps/v1"
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

func isTurbineApp(deployment appsv1.Deployment) bool {
	annotation := readAnnotation(deployment, "turbine/enabled", "false")
	return annotation == "true"
}

func handleClusterPropertyRequest(w http.ResponseWriter, req *http.Request) {
	// print cluster properties to stdout
	clusterPropertiesToStdout(clusterResources)
}

func main() {
	clusterResources = createClients("default")
	r := mux.NewRouter()
	r.HandleFunc("/webhook", handleHookRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment", handleDeploymentRequest).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/deployment/{application}", handleDeploymentDeleteRequest).Methods(http.MethodDelete)

	// cluster properties
	r.HandleFunc("/", handleClusterPropertyRequest).Methods(http.MethodGet)

	log.Fatal(http.ListenAndServe(":8080", r))
}
