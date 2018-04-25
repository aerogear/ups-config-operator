package main

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"

	"log"
	"github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/pkg/api/v1"
	"math/rand"
	"time"
	"fmt"
	"strings"
)

var k8client *kubernetes.Clientset
var pushClient *upsClient

const NamespaceKey = "NAMESPACE"
const ActionAdded = "ADDED"
const ActionDeleted = "DELETED"
const SecretTypeKey = "secretType"
const ServiceInstanceIdKey = "serviceInstanceID"

const BindingSecretType = "mobile-client-binding-secret"
const BindingAppType = "appType"
const BindingClientId = "clientId"
const BindingGoogleKey = "googleKey"
const BindingProjectNumber = "projectNumber"

const UpsSecretName = "unified-push-server"
const UpsURI = "uri"
const IOSCert = "cert"
const IOSPassPhrase = "passphrase"

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// This is required because importing core/v1/Secret leads to a double import and redefinition
// of log_dir
type BindingSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Data              map[string][]byte `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`
	StringData        map[string]string `json:"stringData,omitempty" protobuf:"bytes,4,rep,name=stringData"`
}

// Create a random identifier of the given length. Useful for randomized resource names
func getRandomIdentifier(length int) string {
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = letters[rand.Intn(len(letters))]
	}

	return string(result)
}

// Deletes a secret
func deleteSecret(name string) {
	err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Delete(name, nil)

	if err != nil {
		log.Print("Error deleting secret", err)
	} else {
		log.Printf("Secret `%s` has been deleted", name)
	}
}

func handleAndroidVariant(secret *BindingSecret) {
	// Only instantiate the push client here because we need to wait for the ups secret to
	// be available
	if pushClient == nil {
		pushClient = pushClientOrDie()
	}

	clientId := string(secret.Data[BindingClientId])
	googleKey := string(secret.Data[BindingGoogleKey])
	projectNumber := string(secret.Data[BindingProjectNumber])

	if pushClient.hasAndroidVariant(googleKey) == nil {
		payload := &androidVariant{
			ProjectNumber: projectNumber,
			GoogleKey:     googleKey,
			variant: variant{
				Name:      clientId,
				VariantID: uuid.NewV4().String(),
				Secret:    uuid.NewV4().String(),
			},
		}

		log.Print("Creating a new android variant", payload)
		success, variant := pushClient.createAndroidVariant(payload)
		if success {
			config, _ := variant.getJson()
			updateConfiguration("android", clientId, variant.VariantID, config)
		} else {
			log.Println("No variant has been created in UPS, skipping config secret")
		}
	} else {
		log.Printf("A variant for google key '%s' already exists", googleKey)
	}
}

func handleIOSVariant(secret *BindingSecret) {
	// Only instantiate the push client here because we need to wait for the ups secret to
	// be available
	if pushClient == nil {
		pushClient = pushClientOrDie()
	}

	clientId := string(secret.Data[BindingClientId])
	cert := string(secret.Data[IOSCert])
	passPhrase := string(secret.Data[IOSPassPhrase])

	certByteArray := []byte (cert)
	payload := &iOSVariant{
		Certificate: certByteArray,
		Passphrase:     passPhrase,
		Production: false, //false for now while testing functionality
		variant: variant{
			Name:      clientId,
			VariantID: uuid.NewV4().String(),
			Secret:    uuid.NewV4().String(),
		},
	}

	success, variant := pushClient.createIOSVariant(payload)
	if success {
		config, _ := variant.getJson()
		updateConfiguration("ios", clientId, variant.VariantID, config)
	} else {
		log.Print("No variant has been created in UPS, skipping config secret")
	}
}

// Deletes a configuration from the config secret and from the UPS server
func handleDeleteVariant(secret *BindingSecret) {
	appType := strings.ToLower(string(secret.Data["appType"]))
	success, variantId := removeConfigFromClientSecret(secret)

	if success {
		pushClient.deleteVariant(appType, variantId)
	}
}

// Find a mobile client bound ups config secret
func findMobileClientConfig(clientId string) *v1.Secret {
	filter := metav1.ListOptions{LabelSelector: fmt.Sprintf("clientId=%s,serviceName=ups", clientId)}
	secrets, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).List(filter)

	if err !=  nil {
		panic(err.Error())
	}

	// No secret exists yet, that's ok, we have to create one
	if len(secrets.Items) == 0 {
		return nil
	}

	// Multiple secrets for the same clientId found, that's an error
	if len(secrets.Items) > 1 {
		panic(fmt.Sprintf("Multiple secrets found for clientId %s", clientId))
	}

	return &secrets.Items[0]
}

// Creates a mobile client bound ups config secret
func createClientConfigSecret(clientId string) *v1.Secret {
	configSecretName := fmt.Sprintf("ups-secret-%s-%s", clientId, getRandomIdentifier(5))

	payload := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: configSecretName,
			Labels: map[string]string{
				"mobile":       "enabled",
				"serviceName":  "ups",

				// Used by the mobile-cli to discover config objects
				"serviceInstanceId": pushClient.serviceInstanceId,
				"clientId": clientId,
			},
		},
		Data: map[string][]byte{
			"config": []byte(fmt.Sprintf("{\"pushServerUrl\":\"%s\"}", pushClient.baseUrl)),
		},
	}

	secret, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Create(&payload)
	if err != nil {
		log.Fatal("Error creating ups config secret", err)
	} else {
		log.Printf("Config secret `%s` for variant created", configSecretName)
	}

	return secret
}

// Removes a platform configuration (e.g. iOS or Android) from the `Data.config` map of a UPS configuration
// secret. If there is only one platform it will delete the whole secret.
func removeConfigFromClientSecret(secret *BindingSecret) (bool, string) {
	clientId := string(secret.Data["clientId"])
	configSecret := findMobileClientConfig(clientId)

	if configSecret == nil {
		log.Printf("Cannot delete configuration for client `%s` because the secret does not exist", clientId)
		return false, ""
	}

	appType := strings.ToLower(string(secret.Data["appType"]))
	log.Printf("Deleting %s configuration from `%s`", appType, clientId)

	// Get the current config
	// Retrieve the current config as an object
	var currentConfig map[string]json.RawMessage
	json.Unmarshal(configSecret.Data["config"], &currentConfig)

	// Get the variant ID before removing the config
	// We need that to delete the variant in UPS
	variantId := getVariantIdFromConfig(string(currentConfig[appType]))

	// If there is only one platform in the configuration we can remove the whole
	// secret (2 because there is another key for the server url)
	if (len(currentConfig) == 2) {
		deleteSecret(configSecret.Name)
		return true, variantId
	} else {
		log.Println("More than one variant available, updating configuration object")

		// Delete the config of the given app type and it's URL annotation
		delete(currentConfig, appType)
		delete(configSecret.Annotations, fmt.Sprintf("variant/%s", appType))

		// Create a string of the new config object
		currentConfigString, err := json.Marshal(currentConfig)
		if err != nil {
			panic(err.Error())
		}

		configSecret.Data["config"] = currentConfigString
		_, err = k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Update(configSecret)
		if err != nil {
			log.Println(err.Error())
		}

		return true, variantId
	}
}

func getVariantIdFromConfig(config string) string {
	configMap := make(map[string]string)
	json.Unmarshal([]byte(config), &configMap)
	return configMap["variantId"]
}

// Updates the `Data.config` map of a UPS configuration secret
// The secret can contain multiple variants (e.g. iOS and Android) but is bound to one mobile client
func updateConfiguration(appType string, clientId string, variantId string, newConfig []byte) {
	configSecret := findMobileClientConfig(clientId)
	if configSecret == nil {
		// No config secret exists for this client yet. Create one.
		configSecret = createClientConfigSecret(clientId)
	}

	// Retrieve the current config as an object
	var currentConfig map[string]json.RawMessage
	json.Unmarshal(configSecret.Data["config"], &currentConfig)

	// Overwrite the old platform config
	currentConfig[appType] = []byte(newConfig)

	// Create a string of the complete config object
	currentConfigString, err := json.Marshal(currentConfig)
	if err != nil {
		panic(err.Error())
	}

	// Set the new config
	configSecret.Data["uri"] = []byte(pushClient.baseUrl)
	configSecret.Data["config"] = currentConfigString
	configSecret.Data["name"] = []byte("ups")
	configSecret.Data["type"] = []byte("AeroGear Unifiedpush Server")

	variantUrl := pushClient.baseUrl + "/#/app/" + pushClient.config.ApplicationId + "/variants/" + variantId
	urlAnnotation := fmt.Sprintf("variant/%s", appType)

	if (configSecret.Annotations == nil) {
		configSecret.Annotations = make(map[string]string)
	}
	configSecret.Annotations[urlAnnotation] = variantUrl

	k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Update(configSecret)
	log.Printf("%s configuration of %s has been updated", appType, clientId)
}

func handleAddSecret(obj runtime.Object) {
	raw, _ := json.Marshal(obj)
	var secret = BindingSecret{}
	json.Unmarshal(raw, &secret)
	if val, ok := secret.Labels[SecretTypeKey]; ok && val == BindingSecretType {
		appType := string(secret.Data[BindingAppType])
		log.Printf("A mobile binding secret of type `%s` was added", appType)

		if appType == "Android" {
			handleAndroidVariant(&secret)
		} else if appType == "IOS" {
			handleIOSVariant(&secret)
		}
		// Always delete the secret after handling it regardless of any new resources
		// was created
		deleteSecret(secret.Name)
	}
}

func handleDeleteSecret(obj runtime.Object) {
	raw, _ := json.Marshal(obj)
	var secret = BindingSecret{}
	json.Unmarshal(raw, &secret)

	for _, ref := range secret.ObjectMeta.OwnerReferences {
		if ref.Kind == "ServiceBinding" {
			handleDeleteVariant(&secret)
			break
		}
	}
}

func watchLoop() {
	events, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Watch(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	for update := range events.ResultChan() {
		switch action := update.Type; action {
		case ActionAdded:
			handleAddSecret(update.Object)
		case ActionDeleted:
			handleDeleteSecret(update.Object)
		default:
			log.Print("Unhandled action:", action)
		}
	}
}

func kubeOrDie(config *rest.Config) *kubernetes.Clientset {
	k8client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return k8client
}

func pushClientOrDie() *upsClient {
	upsSecret, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Get(UpsSecretName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	upsBaseURL := string(upsSecret.Data[UpsURI])
	serviceInstanceId := upsSecret.Labels[ServiceInstanceIdKey]

	return &upsClient{
		config: &pushApplication{
			ApplicationId: string(upsSecret.Data["applicationId"]),
		},
		serviceInstanceId: serviceInstanceId,
		baseUrl: upsBaseURL,
	}
}

func main() {
	rand.Seed(time.Now().Unix())

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	k8client = kubeOrDie(config)

	log.Print("Entering watch loop")

	for {
		watchLoop()
	}
}
