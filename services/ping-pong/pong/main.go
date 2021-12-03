package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/turbine/config", turbineConfigHandler)
	http.HandleFunc("/ping", pingHandler)
	log.Println("pong-service available at localhost:3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func turbineConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintln(`{ "serviceName": "pong-service" }`)))
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hazelcast Turbine PONG! Changed again and again. DEMO TIME\n"))
}
