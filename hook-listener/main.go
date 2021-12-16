package main

import (
	"github.com/gorilla/mux"
	"github.com/jerrinot/turbine-demo/hook-listener/internal"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"log"
	"net/http"
)

func main() {
	internal.CreateClients("default")
	r := mux.NewRouter()
	r.HandleFunc("/webhook", internal.HandleDockerHubHookRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment", internal.HandleListDeploymentRequest).Methods(http.MethodGet)
	r.HandleFunc("/deployment", internal.HandleNewDeploymentRequest).Methods(http.MethodPost)
	r.HandleFunc("/deployment/{application}", internal.HandleDeploymentDeleteRequest).Methods(http.MethodDelete)
	log.Fatal(http.ListenAndServe(":8080", r))
}
