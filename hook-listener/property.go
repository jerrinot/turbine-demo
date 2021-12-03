package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func getPodNumber(podClient v12.PodInterface) int {
	pods, err := podClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	return len(pods.Items)
}

func getNodeNumber(nodeClient v12.NodeInterface) int {
	nodes, err := nodeClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	return len(nodes.Items)
}

func getServiceNumber(serviceClient v12.ServiceInterface) int {
	services, err := serviceClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	return len(services.Items)
}

func clusterPropertiesToStdout(clusterResources *ClientResources) {
	podNumber := getPodNumber(clusterResources.PodClient)
	fmt.Printf("There are %d pods in the cluster\n", podNumber)

	nodeNumber := getNodeNumber(clusterResources.NodeClient)
	fmt.Printf("There are %d nodes in the cluster\n", nodeNumber)

	serviceNumber := getServiceNumber(clusterResources.ServiceClient)
	fmt.Printf("There are %d service in the cluster\n", serviceNumber)
}
