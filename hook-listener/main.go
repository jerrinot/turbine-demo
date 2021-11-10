package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func printRequestHeader(req *http.Request) {
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Printf("%v: %v\n", name, h)
		}
	}
}

func handleHookRequest(w http.ResponseWriter, req *http.Request) {
	printRequestHeader(req)
	switch req.Method {
	case "POST":
		fmt.Printf("POST METHOD\n")
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return
		}
		fmt.Printf("%s\n", reqBody)
	default:
		fmt.Printf("Unexpected HTTP METHOD: %s\n", req.Method)
	}
}

func main() {
	http.HandleFunc("/webhook", handleHookRequest)
	http.ListenAndServe(":8080", nil)
}
