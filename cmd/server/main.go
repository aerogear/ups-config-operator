package main

import (
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"

	aerogear "k8s.io/client-go/pkg/api/v1"

	"github.com/aerogear/ups-sidecar/pkg/apis/mobile/v1alpha1"
	mclient "github.com/aerogear/ups-sidecar/pkg/client/mobile/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func convertSecretToUpsSecret(s *aerogear.Secret) *upsAppData {
	return &upsAppData{
		ApplicationId: string(s.Data["applicationId"]),
	}
}

func addAndroidVariant(labels map[string]string, client *upsClient, name string) {
	if val, ok := labels["googleKey"]; ok {
		payload := &upsVariant{
			ProjectNumber: labels["projectNumber"],
			GoogleKey:     val,
			Name:          name,
		}

		jsonString, _ := json.Marshal(payload)

		if !client.doesVariantWithNameExist(name) {
			client.createVariant(jsonString)
		}
	}
}

func waitForEvent(namespace string, config *rest.Config) {
	// Create Clientset
	clientset, err := mclient.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Create watch channel
	wi, err := clientset.MobileV1alpha1().MobileClients(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	// Query the results from the watch channel
	for update := range wi.ResultChan() {
		updatedClient := update.Object.(*v1alpha1.MobileClient)
		labels := updatedClient.Labels

		if len(labels) > 0 {
			k8client := getKubeClient(config)
			upsSecret, _ := k8client.CoreV1().Secrets(namespace).Get("unified-push-server", metav1.GetOptions{})
			data := convertSecretToUpsSecret(upsSecret)
			client := upsClient{config: data}

			addAndroidVariant(labels, &client, updatedClient.Name)
		}
	}
}

func getKubeClient(config *rest.Config) *kubernetes.Clientset {
	k8client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return k8client
}

func main() {
	namespace := os.Getenv("NAMESPACE")

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Waiting for mobile client events on namespace ", namespace)

	// The channel can get closed at any time, that's why we re-establish it
	// inside the infinite loop
	for {
		waitForEvent(namespace, config)
	}
}
