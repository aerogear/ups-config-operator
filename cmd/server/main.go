package main

import (
	"fmt"

	v1alpha1 "github.com/aerogear/ups-sidecar/pkg/apis/mobile/v1alpha1"
	mclient "github.com/aerogear/ups-sidecar/pkg/client/mobile/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func waitForEvent() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := mclient.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	wi, err := clientset.MobileV1alpha1().MobileClients("myproject").Watch(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println("Before loop")

	for update := range wi.ResultChan() {
		client := update.Object.(*v1alpha1.MobileClient)
		fmt.Println("client", client)
		fmt.Println("client Labels: ", client.Labels)
	}

	fmt.Println("Received result")
}

func main() {
	fmt.Println("Waiting for mobile client events...")
	for {
		waitForEvent()
	}
}
