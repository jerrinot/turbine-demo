package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

const (
	applicationPort = 3000
	turbineConfig   = `{"serviceName": "ping-service"}`
)

func main() {
	http.HandleFunc("/turbine/config", turbineConfigHandler)
	http.HandleFunc("/do-ping-pong", pingPongHandler)
	log.Printf("ping-service available at localhost: %d\n", applicationPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", applicationPort), nil))
}

func turbineConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintln(turbineConfig)))
}

func pingPongHandler(w http.ResponseWriter, r *http.Request) {
	// invocationURL := "http://localhost:8466/v1/call/pong-service/ping"
	invocationURL := "http://pong-service:3000/ping"
	pingRequest, err := http.NewRequest(http.MethodGet, invocationURL, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Failed to create ping request: %v", err)))
		return
	}
	pingResponse := getPingPongResult(pingRequest)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(pingResponse))
}

func getPingPongResult(req *http.Request) string {
	pingResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Sprintln("Failed to ping:", err)
	}
	defer pingResponse.Body.Close()
	bodyBytes, err := ioutil.ReadAll(pingResponse.Body)
	if err != nil {
		return fmt.Sprintln("Failed to read pong response")
	}
	return fmt.Sprint("I sent ping and got back ", string(bodyBytes))
}
