package main

import (
	"encoding/json"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"

	aerogear "k8s.io/client-go/pkg/api/v1"

	"github.com/aerogear/ups-sidecar/pkg/apis/mobile/v1alpha1"
	mclient "github.com/aerogear/ups-sidecar/pkg/client/mobile/clientset/versioned"

	"net/http"

	"io/ioutil"
	"strings"

	"bytes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type upsPayload struct {
	GoogleKey string `json:"googleKey"`
	Name      string `json:"name"`
}

type upsAppData struct {
	ApplicationId string `json:"applicationId"`
	InternalUri   string `json:"internalUri"`
}

func convertSecretToUpsSecret(s *aerogear.Secret) *upsAppData {
	return &upsAppData{
		ApplicationId: string(s.Data["applicationId"]),
		InternalUri:   string(s.Data["internalUri"]),
	}
}

func doesVariantExist(data *upsAppData) bool {
	fmt.Println("Checking if a variant already exists")
	var url = "http://localhost:8080/rest/applications/" + data.ApplicationId + "/android"
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err.Error())
		return true
	} else {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		var stringBody = string(body)
		result := strings.Trim(stringBody, "\n") == "[]"
		return !result
	}
}

func createVariant(data *upsAppData, json []byte) {
	fmt.Println("Creating a new variant in UPS")
	var url = "http://localhost:8080/rest/applications/" + data.ApplicationId + "/android"

	fmt.Println("Sending", string(json), "to", url)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(json))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := http.Client{}
	_, err := client.Do(req)

	if err != nil {
		fmt.Println("UPS request error", err.Error())
	}
}

func addAndroidVariant(labels map[string]string, data *upsAppData, name string) {
	if val, ok := labels["googleKey"]; ok {
		payload := &upsPayload{
			GoogleKey: val,
			Name:      name,
		}

		jsonString, _ := json.Marshal(payload)

		if !doesVariantExist(data) {
			createVariant(data, jsonString)
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
		client := update.Object.(*v1alpha1.MobileClient)
		labels := client.Labels

		if len(labels) > 0 {
			k8client := getKubeClient(config)
			upsSecret, _ := k8client.CoreV1().Secrets(namespace).Get("unified-push-server", metav1.GetOptions{})
			data := convertSecretToUpsSecret(upsSecret)
			addAndroidVariant(labels, data, client.Name)
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
