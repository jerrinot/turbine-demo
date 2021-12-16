package main

import (
	"github.com/gorilla/mux"
	"github.com/jerrinot/turbine-demo/hook-listener/internal"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"log"
	"net/http"
)

func main() {
	controller := internal.NewKubernetesController("default")
	deployment := internal.NewDeployment(controller)
	hubController := internal.NewDockerHubController(controller)

	r := mux.NewRouter()
	r.HandleFunc("/webhook", hubController.HandleDockerHubHookRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment", deployment.HandleListDeploymentRequest).Methods(http.MethodGet)
	r.HandleFunc("/deployment", deployment.HandleNewDeploymentRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment/{application}", deployment.HandleDeploymentDeleteRequest).Methods(http.MethodDelete)
	log.Fatal(http.ListenAndServe(":8080", r))
}
