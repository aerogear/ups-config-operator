package main

import (
	"os"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"

	"log"

	"github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"fmt"
	"math/rand"
	"strings"
	"time"

	"k8s.io/client-go/pkg/api/v1"
	sc "github.com/aerogear/ups-config-operator/pkg/client/servicecatalog/clientset/versioned"
	mc "github.com/aerogear/ups-config-operator/pkg/client/mobile/clientset/versioned"
	"github.com/pkg/errors"
)

var mobileclient *mc.Clientset
var k8client *kubernetes.Clientset
var scclient *sc.Clientset
var pushClient *upsClient

const NamespaceKey = "NAMESPACE"
const ActionAdded = "ADDED"
const ActionDeleted = "DELETED"
const SecretTypeKey = "secretType"
const ServiceInstanceIdKey = "serviceInstanceID"
const ServiceBindingIdKey = "serviceBindingId"
const ServiceInstanceNameKey = "serviceInstanceName"

const BindingSecretType = "mobile-client-binding-secret"
const BindingAppType = "appType"
const BindingClientId = "clientId"
const BindingGoogleKey = "googleKey"
const BindingProjectNumber = "projectNumber"

const UpsSecretName = "unified-push-server"
const UpsURI = "uri"
const IOSCert = "cert"
const IOSPassPhrase = "passphrase"
const IOSIsProduction = "isProduction"

// time in seconds
const UPSPollingInterval = 5

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
	serviceBindingId := string(secret.Data[ServiceBindingIdKey])
	serviceInstanceName := string(secret.Data[ServiceInstanceNameKey])

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
			updateConfiguration("android", clientId, variant.VariantID, config, serviceBindingId, serviceInstanceName)
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
	serviceBindingId := string(secret.Data[ServiceBindingIdKey])
	serviceInstanceName := string(secret.Data[ServiceInstanceNameKey])
	isProductionString := string(secret.Data[IOSIsProduction])
	isProduction, err := strconv.ParseBool(isProductionString)

	if err != nil {
		log.Printf("iOS variant with clientId %v is invalid, isProduction value %v should be true or false. Setting to false", clientId, isProductionString)
		isProduction = false
	}

	certByteArray := []byte(cert)
	payload := &iOSVariant{
		Certificate: certByteArray,
		Passphrase:  passPhrase,
		Production:  isProduction, //false for now while testing functionality
		variant: variant{
			Name:      clientId,
			VariantID: uuid.NewV4().String(),
			Secret:    uuid.NewV4().String(),
		},
	}

	success, variant := pushClient.createIOSVariant(payload)
	if success {
		config, _ := variant.getJson()
		updateConfiguration("ios", clientId, variant.VariantID, config, serviceBindingId, serviceInstanceName)
	} else {
		log.Print("No variant has been created in UPS, skipping config secret")
	}
}

// Deletes a configuration from the config secret and from the UPS server
func handleDeleteVariant(secret *BindingSecret) {
	appType := strings.ToLower(string(secret.Data["appType"]))
	success, variantId := removeConfigFromClientSecret(secret, appType)

	if success {
		success := pushClient.deleteVariant(appType, variantId)
		if !success {
			log.Printf("UPS reported an error when deleting variant %s", variantId)
		}
	}
}

// Find a service binding by its ExternalID
func getServiceBindingNameByID(bindingId string) (string, error) {
	// Get a list of all service bindings in the namespace and find the one with a matching ExternalID
	// This is not very efficient and could be improved with a jsonpath query but it looks like client-go
	// does not support jsonpath or at least I could not find any examples.
	bindings, err := scclient.ServicecatalogV1beta1().ServiceBindings(os.Getenv(NamespaceKey)).List(metav1.ListOptions{})
	if err != nil {
		return "", errors.New("Error listing service bindings")
	}

	for _, binding := range bindings.Items {
		log.Printf("Checking service binding %s", binding.Name)
		if binding.Spec.ExternalID == bindingId {
			return binding.Name, nil
		}
	}

	return "", errors.New(fmt.Sprintf("Can't find a binding with ExternalID %s", bindingId))
}

func deleteServiceBinding(bindingName string) {
	err := scclient.ServicecatalogV1beta1().ServiceBindings(os.Getenv(NamespaceKey)).Delete(bindingName, nil)
	if err != nil {
		log.Printf("Error deleting service binding instance %s", bindingName)
	}
}

// Find a mobile client bound ups config secret
func findMobileClientConfig(clientId string) *v1.Secret {
	filter := metav1.ListOptions{LabelSelector: fmt.Sprintf("clientId=%s,serviceName=ups", clientId)}
	secrets, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).List(filter)

	if err != nil {
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

// Creates the JSON string for the mobile-client variant annotation
func generateVariantAnnotationValue(url string, appType string) ([]byte, error) {
	annotation := VariantAnnotation{
		 Type: "href",
		 Label: fmt.Sprintf("UPS %s Variant", appType),
		 Value: url,
	}

	return json.Marshal(annotation)
}

// Adds an annotation to the mobile client that contains information about this variant
// (currently URL and Name)
func addAnnotationToMobileClient(clientId string, appType string, variantUrl string, serviceInstanceName string) {
	client, err := mobileclient.MobileV1alpha1().MobileClients(os.Getenv(NamespaceKey)).Get(clientId, metav1.GetOptions{})
	if err != nil {
		log.Printf("No mobile client with name %s found", clientId)
		return
	}

	annotationName := fmt.Sprintf("org.aerogear.binding.%s/variant-%s", serviceInstanceName, appType)
	annotationValue, err := generateVariantAnnotationValue(variantUrl, appType)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if client.Annotations == nil {
		client.Annotations = make(map[string]string)
	}

	client.Annotations[annotationName] = string(annotationValue)
	_, err = mobileclient.MobileV1alpha1().MobileClients(os.Getenv(NamespaceKey)).Update(client)
	if err != nil {
		log.Printf(err.Error())
	}
}

func removeAnnotationFromMobileClient(clientId string, appType string, serviceInstanceName string) {
	client, err := mobileclient.MobileV1alpha1().MobileClients(os.Getenv(NamespaceKey)).Get(clientId, metav1.GetOptions{})
	if err != nil {
		log.Printf("No mobile client with name %s found", clientId)
		return
	}

	if client.Annotations != nil {
		annotationName := fmt.Sprintf("org.aerogear.binding.%s/variant-%s", serviceInstanceName, appType)
		log.Printf("Removing annotation %s from mobile client %s", annotationName, clientId)

		delete(client.Annotations, annotationName)
		_, err = mobileclient.MobileV1alpha1().MobileClients(os.Getenv(NamespaceKey)).Update(client)
		if err != nil {
			log.Printf(err.Error())
		}
	}
}

// Creates a mobile client bound ups config secret
func createClientConfigSecret(clientId string, serviceInstanceName string) *v1.Secret {
	configSecretName := fmt.Sprintf("ups-secret-%s-%s", clientId, getRandomIdentifier(5))

	payload := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: configSecretName,
			Labels: map[string]string{
				"mobile":      "enabled",
				"serviceName": "ups",

				// Used by the mobile-cli to discover config objects
				"serviceInstanceId": pushClient.serviceInstanceId,
				"clientId":          clientId,
				"pushApplicationId": pushClient.config.ApplicationId,
			},
		},
		Data: map[string][]byte{
			// Used to generate the name of the UI annotations
			ServiceInstanceNameKey: []byte(serviceInstanceName),
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
func removeConfigFromClientSecret(secret *BindingSecret, appType string) (bool, string) {
	clientId := string(secret.Data["clientId"])
	configSecret := findMobileClientConfig(clientId)

	if configSecret == nil {
		log.Printf("Cannot delete configuration for client `%s` because the secret does not exist", clientId)
		return false, ""
	}

	serviceInstanceName := string(configSecret.Data[ServiceInstanceNameKey])
	log.Printf("Deleting %s configuration from %s", appType, clientId)

	// Remove the annotation also from the mobile client
	removeAnnotationFromMobileClient(clientId, appType, serviceInstanceName)

	// Get the current config
	// Retrieve the current config as an object
	var currentConfig map[string]json.RawMessage
	json.Unmarshal(configSecret.Data["config"], &currentConfig)

	// Get the variant ID before removing the config
	// We need that to delete the variant in UPS
	variantId := getVariantIdFromConfig(string(currentConfig[appType]))

	// If there is only one platform in the configuration we can remove the whole
	// secret (2 because there is another key for the server url)
	if len(currentConfig) == 2 {
		deleteSecret(configSecret.Name)
		return true, variantId
	} else {
		log.Println("More than one variant available, updating configuration object")

		// Delete the config of the given app type and it's annotations
		delete(currentConfig, appType)
		delete(configSecret.Annotations, fmt.Sprintf("binding/%s", appType))

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
func updateConfiguration(appType string, clientId string, variantId string, newConfig []byte, bindingId string, serviceInstanceName string) {
	configSecret := findMobileClientConfig(clientId)
	if configSecret == nil {
		// No config secret exists for this client yet. Create one.
		configSecret = createClientConfigSecret(clientId, serviceInstanceName)
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

	// Add the binding annotation to the UPS secret: this is done to link the actual ServiceBinding
	// Instance back to this secret. In case the variant is deleted in UPS we can use this ID to delete
	// the service binding
	bindingAnnotation := fmt.Sprintf("binding/%s", appType)
	if configSecret.Annotations == nil {
		configSecret.Annotations = make(map[string]string)
	}
	configSecret.Annotations[bindingAnnotation] = bindingId

	// Annotate the mobile client with the variant URL. This is done to display a link to
	// the variant in the Mobile Client UI in Openshift
	variantUrl := pushClient.baseUrl + "/#/app/" + pushClient.config.ApplicationId + "/variants/" + variantId
	addAnnotationToMobileClient(clientId, appType, variantUrl, serviceInstanceName)

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

func enterWatchLoop() {
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

func clientsOrDie(config *rest.Config) {
	k8client = kubernetes.NewForConfigOrDie(config)
	scclient = sc.NewForConfigOrDie(config)
	mobileclient = mc.NewForConfigOrDie(config)
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
		baseUrl:           upsBaseURL,
	}
}

func getUPSClientConfigsFromSecrets(secrets []v1.Secret) []map[string]string {
	results := []map[string]string{}

	for _, secret := range secrets {
		log.Printf("processing secret: %v", secret)

		// Retrieve the current config as an object
		clientConfig := UPSClientConfig{}
		json.Unmarshal(secret.Data["config"], &clientConfig)

		if clientConfig.Android != nil {
			androidConfig := *clientConfig.Android
			results = append(results, map[string]string{
				"variantId":        androidConfig["variantId"],
				"servicebindingId": secret.ObjectMeta.Annotations["binding/android"],
			})
		}

		if clientConfig.IOS != nil {
			iOSConfig := *clientConfig.IOS
			results = append(results, map[string]string{
				"variantId":        iOSConfig["variantId"],
				"servicebindingId": secret.ObjectMeta.Annotations["binding/ios"],
			})
		}
	}

	return results
}

func getUPSSecrets() ([]v1.Secret, error) {
	selector := fmt.Sprintf("serviceName=ups,pushApplicationId=%s", pushClient.config.ApplicationId)
	filter := metav1.ListOptions{LabelSelector: selector}
	secretsList, error := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).List(filter)
	return secretsList.Items, error
}

func compareUPSVariantsWithVariantsFromSecrets() {

	if pushClient == nil {
		pushClient = pushClientOrDie()
	}

	secrets, err := getUPSSecrets()

	// then process these into a list of variants
	clientConfigs := getUPSClientConfigsFromSecrets(secrets)
	log.Printf("Processed variants from secret list %v", clientConfigs)

	if err != nil {
		log.Printf("Error searching for ups secrets: %v", err.Error())
		return
	}

	UPSVariants, err := pushClient.getVariants()

	if err != nil {
		log.Printf("An error occurred trying to get variants from UPS service: %v", err.Error())
		return
	}

	log.Printf("got variants from UPS: %v", UPSVariants)

	for _, config := range clientConfigs {
		variantId := config["variantId"]
		found := false
		for _, variant := range UPSVariants {
			if variant.VariantID == variantId {
				found = true
			}
		}
		if !found {
			fmt.Println("variant Id %v found in client configs but not found in UPS. Should delete", variantId)
		}
	}
}

func startPollingUPS() {
	interval := UPSPollingInterval * time.Second
	for {
		<-time.After(interval)
		log.Println("Should Poll UPS now")
		compareUPSVariantsWithVariantsFromSecrets()
	}
}

func main() {
	rand.Seed(time.Now().Unix())

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientsOrDie(config)

	log.Print("Entering watch loop")

	go startPollingUPS()
	enterWatchLoop()
}
