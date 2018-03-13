package main

import (
	"fmt"
	"os"

	v1alpha1 "github.com/aerogear/ups-sidecar/pkg/apis/mobile/v1alpha1"
	mclient "github.com/aerogear/ups-sidecar/pkg/client/mobile/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func main() {

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
	wi, err := clientset.MobileV1alpha1().MobileClients("test").Watch(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for update := range wi.ResultChan() {
		client := update.Object.(*v1alpha1.MobileClient)
		fmt.Println("update", client)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
