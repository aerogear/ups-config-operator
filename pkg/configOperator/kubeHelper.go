package configOperator

import (
	"github.com/aerogear/ups-config-operator/pkg/constants"
	"fmt"
	"os"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/kubernetes"
	"log"
	"github.com/pkg/errors"

	sc "github.com/aerogear/mobile-crd-client/pkg/client/servicecatalog/clientset/versioned"
	"math/rand"
	"k8s.io/apimachinery/pkg/watch"
)

type KubeHelper struct {
	k8client *kubernetes.Clientset
	scclient *sc.Clientset
}

func NewKubeHelper(k8client *kubernetes.Clientset, scclient *sc.Clientset) *KubeHelper {
	helper := new(KubeHelper)

	helper.k8client = k8client
	helper.scclient = scclient

	return helper
}

func (helper KubeHelper) startSecretWatch() (watch.Interface, error) {
	return helper.k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).Watch(metav1.ListOptions{})
}

func (helper KubeHelper) listSecrets(selector string) (*v1.SecretList, error) {
	filter := metav1.ListOptions{LabelSelector: selector}
	return helper.k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).List(filter)
}

// Find a mobile client bound ups config secret
func (helper KubeHelper) findMobileClientConfig(clientId string) *v1.Secret {
	filter := metav1.ListOptions{LabelSelector: fmt.Sprintf("clientId=%s,serviceName=ups", clientId)}
	secrets, err := helper.k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).List(filter)

	// TODO: remove error handling here!
	if err != nil {
		panic(err.Error())
	}

	// No secret exists yet, that's ok, we have to create one
	if len(secrets.Items) == 0 {
		return nil
	}

	// TODO: remove error handling here!
	// Multiple secrets for the same clientId found, that's an error
	if len(secrets.Items) > 1 {
		panic(fmt.Sprintf("Multiple secrets found for clientId %s", clientId))
	}

	return &secrets.Items[0]
}

// Find a service binding by its ExternalID
func (helper KubeHelper) getServiceBindingNameByID(bindingId string) (string, error) {
	// Get a list of all service bindings in the namespace and find the one with a matching ExternalID
	// This is not very efficient and could be improved with a jsonpath query but it looks like client-go
	// does not support jsonpath or at least I could not find any examples.
	bindings, err := helper.scclient.ServicecatalogV1beta1().ServiceBindings(os.Getenv(constants.EnvVarKeyNamespace)).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	for _, binding := range bindings.Items {
		log.Printf("Checking service binding %s", binding.Name)
		if binding.Spec.ExternalID == bindingId {
			return binding.Name, nil
		}
	}

	return "", errors.New(fmt.Sprintf("Can't find a binding with ExternalID %s", bindingId))
}

func (helper KubeHelper) deleteServiceBinding(bindingName string) error {
	return helper.scclient.ServicecatalogV1beta1().ServiceBindings(os.Getenv(constants.EnvVarKeyNamespace)).Delete(bindingName, nil)
}

// Creates a mobile client bound ups config secret
func (helper KubeHelper) createClientConfigSecret(clientId string, serviceInstanceName string, serviceInstanceId string, pushAppId string) *v1.Secret {
	configSecretName := fmt.Sprintf("ups-secret-%s-%s", clientId, getRandomIdentifier(5))

	payload := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: configSecretName,
			Labels: map[string]string{
				"mobile":      "enabled",
				"serviceName": "ups",

				// Used by the mobile-cli to discover config objects
				"serviceInstanceId": serviceInstanceId,
				"clientId":          clientId,
				"pushApplicationId": pushAppId,
			},
		},
		Data: map[string][]byte{
			// Used to generate the name of the UI annotations
			constants.BindingDataServiceInstanceNameKey: []byte(serviceInstanceName),
			"config":                                    []byte("{}"),
		},
	}

	secret, err := helper.k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).Create(&payload)

	// TODO: remove error handling here!
	if err != nil {
		log.Fatal("Error creating ups config secret", err)
	} else {
		log.Printf("Config secret `%s` for variant created", configSecretName)
	}

	return secret
}

func (helper KubeHelper) updateSecret(secret *v1.Secret) (*v1.Secret, error) {
	return helper.k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).Update(secret)
}

// Deletes a secret
func (helper KubeHelper) deleteSecret(name string) {
	err := helper.k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).Delete(name, nil)

	// TODO: remove error handling here!
	if err != nil {
		log.Print("Error deleting secret", err)
	} else {
		log.Printf("Secret `%s` has been deleted", name)
	}
}

// Create a random identifier of the given length. Useful for randomized resource names
func getRandomIdentifier(length int) string {
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = letters[rand.Intn(len(letters))]
	}

	return string(result)
}
