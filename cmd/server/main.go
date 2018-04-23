package main

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"

	"log"
	"github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	mobile "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/pkg/api/v1"
	"math/rand"
	"time"

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
const AppType = "type"
const IOSCert = "cert"
const IOSPassPhrase = "passphrase"
const VariantReferenceId = "variantReferenceId" // this is a specific id for deleting resources such as secrets and configmaps - this is NOT the UPS variant id

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// This is required because importing core/v1/Secret leads to a double import and redefinition
// of log_dir
type BindingSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Data              map[string][]byte `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`
	StringData        map[string]string `json:"stringData,omitempty" protobuf:"bytes,4,rep,name=stringData"`
}

func getRandomIdentifier(length int) string {
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = letters[rand.Intn(len(letters))]
	}

	return string(result)
}

// Deletes the binding secret after the sync operation has completed
func deleteSecret(name string) {
	err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Delete(name, nil)
	if err != nil {
		log.Fatal("Error creating config map", err)
	} else {
		log.Printf("Secret `%s` has been deleted", name)
	}
}

func createAndroidVariantConfigMap(variant *androidVariant, clientId string, variantReferenceId string) {
	//initialise the UPS data which will be used for the configmap value
	var variantUrl = pushClient.baseUrl + "/#/app/" + pushClient.config.ApplicationId + "/variants/" + variant.VariantID

	// The name of the config map needs to have a random element because there could be
	// more than one config map per client
	variantName := variant.Name + "-config-map-" + getRandomIdentifier(5)

	payload := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: variantName,
			Labels: map[string]string{
				"mobile":       "enabled",
				"serviceName":  "ups",
				"resourceType": "binding",

				// Used by the mobile-cli to discover config objects
				"serviceInstanceId": pushClient.serviceInstanceId,
				"mobileClientID": clientId,
			},
		},
		Data: map[string]string{
			"name":          variant.Name,
			"description":   variant.Description,
			"variantID":     variant.VariantID,
			"secret":        variant.Secret,
			"googleKey":     variant.GoogleKey,
			"projectNumber": variant.ProjectNumber,
			"type":          "android",
			"variantURL":    variantUrl,
			"variantReferenceId": variantReferenceId,
		},
	}
	_, err := k8client.CoreV1().ConfigMaps(os.Getenv(NamespaceKey)).Create(&payload)
	if err != nil {
		log.Fatal("Error creating config map", err)
	} else {
		log.Printf("Config map `%s` for variant created", variantName)
	}
}

func createIOSVariantConfigMap(variant *iOSVariant, clientId string, variantReferenceId string) {
	//initialise the UPS data which will be used for the configmap value
	var variantUrl = pushClient.baseUrl + "/#/app/" + pushClient.config.ApplicationId + "/variants/" + variant.VariantID

	log.Print("ups variant url : ", variantUrl)

	// The name of the config map needs to have a random element because there could be
	// more than one config map per client
	variantName := variant.Name + "-config-map-" + getRandomIdentifier(5)

	production := "true"
	if !variant.Production {
		production = "false"
	}
	payload := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: variantName,
			Labels: map[string]string{
				"mobile":       "enabled",
				"serviceName":  "ups",
				"resourceType": "binding",

				// Used by the mobile-cli to discover config objects
				"serviceInstanceId": pushClient.serviceInstanceId,
				"mobileClientID": clientId,
			},
		},
		Data: map[string]string{
			"name":          variant.Name,
			"description":   variant.Description,
			"variantID":     variant.VariantID,
			"secret":        variant.Secret,
		    "production":	production,
			"type":          "ios",
			"variantURL":    variantUrl,
			"variantReferenceId": variantReferenceId,
		},
	}
	_, err := k8client.CoreV1().ConfigMaps(os.Getenv(NamespaceKey)).Create(&payload)
	if err != nil {
		log.Fatal("Error creating config map", err)
	} else {
		log.Printf("Config map `%s` for variant created", variantName)
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
	variantReferenceId := string(secret.Data[VariantReferenceId])

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
			createAndroidVariantConfigMap(variant, clientId, variantReferenceId)
		} else {
			log.Fatal("No variant has been created in UPS, skipping config map")
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
	variantReferenceId := string(secret.Data[VariantReferenceId])

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
		createIOSVariantConfigMap(variant, clientId, variantReferenceId)
	} else {
		log.Print("No variant has been created in UPS, skipping config map")
	}
}

func handleDeleteVariant(secret *BindingSecret) {
	if _, ok := secret.Data[VariantReferenceId]; !ok {
		log.Println("Secret does not contain a variant reference id, can't delete the variant")
		return
	}
	variantReferenceId := string(secret.Data[VariantReferenceId])

	// Get all config maps
	configs, err := k8client.CoreV1().ConfigMaps(os.Getenv(NamespaceKey)).List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	configMapDeleted := false
	var variantId string  // UPS variant id of the variant to be deleted
    var	appType string

	//Filter config maps to identify the one associated with the given variant reference id
	for _, config := range configs.Items {
		if config.Labels["resourceType"] == "binding" && config.Data[VariantReferenceId] == variantReferenceId {
			name := config.Name
			log.Printf("Config map with name `%s` has a matching variant reference id", name)
			variantId = string(config.Data["variantID"])
			appType = string(config.Data[AppType])
			// Delete the config map
			err := k8client.CoreV1().ConfigMaps(os.Getenv(NamespaceKey)).Delete(name, nil)
			if err != nil {
				log.Fatal("Error deleting config map with name `%s`", name, err)
				break
			}
			configMapDeleted = true
			log.Printf("Config map `%s` has been deleted", name)
			break
		}
	}

	if pushClient == nil {
		pushClient = pushClientOrDie()
	}

	// Delete the UPS variant only if the associated config map has been deleted
	if configMapDeleted == true {
		pushClient.deleteVariant(appType, variantId)
	}
}

func handleAddSecret(obj runtime.Object) {
	raw, _ := json.Marshal(obj)
	var secret = BindingSecret{}
	json.Unmarshal(raw, &secret)
	if val, ok := secret.Labels[SecretTypeKey]; ok && val == BindingSecretType {
		appType := string(secret.Data[BindingAppType])

		if appType == "Android" {
			log.Print("A mobile binding secret of type `Android` was added")
			handleAndroidVariant(&secret)
		} else if appType == "IOS" {
			log.Print("A mobile binding secret of type `IOS` was added")
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

func convertSecretToUpsSecret(s *mobile.Secret) *pushApplication {
	return &pushApplication{
		ApplicationId: string(s.Data["applicationId"]),
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
		config: convertSecretToUpsSecret(upsSecret),
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
