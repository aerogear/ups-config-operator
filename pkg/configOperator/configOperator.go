package configOperator

import (
	"strconv"

	"encoding/json"

	"log"

	"github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/runtime"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/pkg/api/v1"
	"github.com/aerogear/ups-config-operator/pkg/constants"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

type ConfigOperator struct {
	pushClient       UpsClient
	annotationHelper AnnotationHelper
	kubeHelper       KubeHelper
}

func NewConfigOperator(pushClient UpsClient, annotationHelper AnnotationHelper, kubeHelper KubeHelper) *ConfigOperator {
	op := new(ConfigOperator)

	op.pushClient = pushClient
	op.annotationHelper = annotationHelper
	op.kubeHelper = kubeHelper

	return op
}

func (op ConfigOperator) StartService() {
	log.Print("Entering watch loop")

	go op.startPollingUPS()
	op.startKubeWatchLoop()
}

// startPollingUPS() is a loop that calls compareUPSVariantsWithClientConfigs() in intervals
func (op ConfigOperator) startPollingUPS() {
	interval := constants.UPSPollingInterval * time.Second
	for {
		<-time.After(interval)
		op.compareUPSVariantsWithClientConfigs()
	}
}

func (op ConfigOperator) startKubeWatchLoop() {
	events, err := op.kubeHelper.startSecretWatch()
	if err != nil {
		panic(err.Error())
	}

	for update := range events.ResultChan() {
		switch action := update.Type; action {
		case constants.K8SecretEventTypeAdded:
			op.handleAddSecret(update.Object)
		case constants.K8SecretEventTypeDeleted:
			op.handleDeleteSecret(update.Object)
		default:
			log.Print("Unhandled action:", action)
		}
	}
}

func (op ConfigOperator) handleAddSecret(obj runtime.Object) {
	raw, _ := json.Marshal(obj)
	var secret = BindingSecret{}
	json.Unmarshal(raw, &secret)
	if val, ok := secret.Labels[constants.SecretTypeLabelKey]; ok && val == constants.BindingSecretTypeMobile {
		appType := string(secret.Data[constants.BindingDataAppTypeKey])
		log.Printf("A mobile binding secret of type `%s` was added", appType)

		if appType == "Android" {
			op.handleAndroidVariant(&secret)
		} else if appType == "IOS" {
			op.handleIOSVariant(&secret)
		}
		// Always delete the secret after handling it regardless of any new resources
		// was created
		op.kubeHelper.deleteSecret(secret.Name)
	}
}

func (op ConfigOperator) handleDeleteSecret(obj runtime.Object) {
	raw, _ := json.Marshal(obj)
	var secret = BindingSecret{}
	json.Unmarshal(raw, &secret)

	for _, ref := range secret.ObjectMeta.OwnerReferences {
		if ref.Kind == "ServiceBinding" {
			op.handleDeleteVariant(&secret)
			break
		}
	}
}

// compareUPSVariantsWithClientConfigs() compares the UPS client configs stored in k8's secrets
// against the variants in UPS in order to detect if a variant has been deleted in UPS
// If a client config is found that references a variant not found in UPS then we clean up the client config by deleting the associated servicebinding.
func (op ConfigOperator) compareUPSVariantsWithClientConfigs() {
	// get the UPS related secrets
	selector := fmt.Sprintf("serviceName=ups,pushApplicationId=%s", op.pushClient.getApplicationId())
	secretsList, err := op.kubeHelper.listSecrets(selector)
	secrets:= secretsList.Items

	if err != nil {
		log.Printf("Error searching for ups secrets: %v", err.Error())
		return
	}

	// process the secrets into a list of VariantServiceBindingMappings
	// each element has VariantId and ServiceBindingId
	clientConfigs := op.getUPSVariantServiceBindingMappings(secrets)

	// Get all variants from UPS
	UPSVariants, err := op.pushClient.getVariants()

	if err != nil {
		log.Printf("An error occurred trying to get variants from UPS service: %v", err.Error())
		return
	}

	for _, clientConfig := range clientConfigs {
		found := false

		for _, variant := range UPSVariants {
			if variant.VariantID == clientConfig.VariantId {
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("variant Id %v found in client configs but not found in UPS. Should delete", clientConfig.VariantId)
			err := op.handleDeleteServiceBinding(clientConfig.ServiceBindingId)
			if err != nil {
				log.Printf("Error deleting service binding instance with id %s\n%s", clientConfig.ServiceBindingId, err.Error())
			}
		}
	}
}

// getUPSVariantServiceBindingMappings() takes the list of secrets and returns a list of VariantServiceBindingMappings
func (op ConfigOperator) getUPSVariantServiceBindingMappings(secrets []v1.Secret) []VariantServiceBindingMapping {

	var results []VariantServiceBindingMapping

	buildAndAppendResult := func(results []VariantServiceBindingMapping, variantId string, serviceBindingId string, secret v1.Secret) []VariantServiceBindingMapping {
		if variantServiceBindingMapping, err := GetClientConfigRepresentation(variantId, serviceBindingId); err != nil {
			log.Printf("invalid android UPS client config found in secret %s reason: %s", secret.Name, err.Error())
			return results
		} else {
			return append(results, variantServiceBindingMapping)
		}
	}

	for _, secret := range secrets {

		// Retrieve the current config as an object
		clientConfig := UPSClientConfig{}
		json.Unmarshal(secret.Data["config"], &clientConfig)

		if clientConfig.Android != nil {
			androidConfig := *clientConfig.Android
			variantId := androidConfig["variantId"]
			serviceBindingId := secret.ObjectMeta.Annotations["binding/android"]
			results = buildAndAppendResult(results, variantId, serviceBindingId, secret)
		}

		if clientConfig.IOS != nil {
			iOSConfig := *clientConfig.IOS
			variantId := iOSConfig["variantId"]
			serviceBindingId := secret.ObjectMeta.Annotations["binding/ios"]
			results = buildAndAppendResult(results, variantId, serviceBindingId, secret)
		}
	}
	return results
}

func (op ConfigOperator) handleDeleteServiceBinding(servicebindingId string) error {
	serviceBindingName, err := op.kubeHelper.getServiceBindingNameByID(servicebindingId)
	if err != nil {
		return err
	}
	err = op.kubeHelper.deleteServiceBinding(serviceBindingName)
	return err
}

func (op ConfigOperator) handleAndroidVariant(secret *BindingSecret) {
	clientId := string(secret.Data[constants.BindingDataClientIdKey])
	googleKey := string(secret.Data[constants.BindingDataGoogleKey])
	projectNumber := string(secret.Data[constants.BindingDataProjectNumberKey])
	serviceBindingId := string(secret.Data[constants.BindingDataServiceBindingIdKey])
	serviceInstanceName := string(secret.Data[constants.BindingDataServiceInstanceNameKey])

	payload := &AndroidVariant{
		ProjectNumber: projectNumber,
		GoogleKey:     googleKey,
		Variant: Variant{
			Name:      clientId,
			VariantID: uuid.NewV4().String(),
			Secret:    uuid.NewV4().String(),
		},
	}

	log.Print("Creating a new android variant", payload)
	success, variant := op.pushClient.createAndroidVariant(payload)
	if success {
		config, _ := variant.getJson()
		op.updateConfiguration("android", clientId, variant.VariantID, config, serviceBindingId, serviceInstanceName)
	} else {
		log.Println("No variant has been created in UPS, skipping config secret")
	}
}

func (op ConfigOperator) handleIOSVariant(secret *BindingSecret) {
	clientId := string(secret.Data[constants.BindingDataClientIdKey])
	cert := string(secret.Data[constants.BindingDataIOSCertKey])
	passPhrase := string(secret.Data[constants.BindingDataIOSPassPhraseKey])
	serviceBindingId := string(secret.Data[constants.BindingDataServiceBindingIdKey])
	serviceInstanceName := string(secret.Data[constants.BindingDataServiceInstanceNameKey])
	isProductionString := string(secret.Data[constants.BindingDataIOSIsProductionKey])
	isProduction, err := strconv.ParseBool(isProductionString)

	if err != nil {
		log.Printf("iOS variant with clientId %v is invalid, isProduction value %v should be true or false. Setting to false", clientId, isProductionString)
		isProduction = false
	}

	certByteArray := []byte(cert)
	payload := &IOSVariant{
		Certificate: certByteArray,
		Passphrase:  passPhrase,
		Production:  isProduction, //false for now while testing functionality
		Variant: Variant{
			Name:      clientId,
			VariantID: uuid.NewV4().String(),
			Secret:    uuid.NewV4().String(),
		},
	}

	success, variant := op.pushClient.createIOSVariant(payload)
	if success {
		config, _ := variant.getJson()
		op.updateConfiguration("ios", clientId, variant.VariantID, config, serviceBindingId, serviceInstanceName)
	} else {
		log.Print("No variant has been created in UPS, skipping config secret")
	}
}

// Deletes a configuration from the config secret and from the UPS server
func (op ConfigOperator) handleDeleteVariant(secret *BindingSecret) {
	appType := strings.ToLower(string(secret.Data["appType"]))
	success, variantId := op.removeConfigFromClientSecret(secret, appType)

	if success {
		success := op.pushClient.deleteVariant(appType, variantId)
		if !success {
			log.Printf("UPS reported an error when deleting variant %s", variantId)
		}
	}
}

// Removes a platform configuration (e.g. iOS or Android) from the `Data.config` map of a UPS configuration
// secret. If there is only one platform it will delete the whole secret.
func (op ConfigOperator) removeConfigFromClientSecret(secret *BindingSecret, appType string) (bool, string) {
	clientId := string(secret.Data["clientId"])

	if clientId == "" {
		// this secret is not the secret we're looking for
		return false, ""
	}

	configSecret := op.kubeHelper.findMobileClientConfig(clientId)

	if configSecret == nil {
		log.Printf("Cannot delete configuration for client `%s` because the secret does not exist", clientId)
		return false, ""
	}

	serviceInstanceName := string(configSecret.Data[constants.BindingDataServiceInstanceNameKey])
	log.Printf("Deleting %s configuration from %s", appType, clientId)

	// Remove the annotation also from the mobile client
	op.annotationHelper.removeAnnotationFromMobileClient(clientId, appType, serviceInstanceName)

	// Get the current config
	// Retrieve the current config as an object
	var currentConfig map[string]json.RawMessage
	json.Unmarshal(configSecret.Data["config"], &currentConfig)

	// Get the variant ID before removing the config
	// We need that to delete the variant in UPS
	variantId := op.getVariantIdFromConfig(string(currentConfig[appType]))

	// If there is only one platform in the configuration we can remove the whole
	// secret
	if len(currentConfig) == 1 {
		op.kubeHelper.deleteSecret(configSecret.Name)
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
		_, err = op.kubeHelper.updateSecret(configSecret)
		if err != nil {
			log.Println(err.Error())
		}

		return true, variantId
	}
}

func (op ConfigOperator) getVariantIdFromConfig(config string) string {
	configMap := make(map[string]string)
	json.Unmarshal([]byte(config), &configMap)
	return configMap["variantId"]
}

// Updates the `Data.config` map of a UPS configuration secret
// The secret can contain multiple variants (e.g. iOS and Android) but is bound to one mobile client
func (op ConfigOperator) updateConfiguration(appType string, clientId string, variantId string, newConfig []byte, bindingId string, serviceInstanceName string) {
	configSecret := op.kubeHelper.findMobileClientConfig(clientId)
	if configSecret == nil {
		// No config secret exists for this client yet. Create one.
		configSecret = op.kubeHelper.createClientConfigSecret(clientId, serviceInstanceName, op.pushClient.getServiceInstanceId(), op.pushClient.getApplicationId())
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
	configSecret.Data["uri"] = []byte(op.pushClient.getBaseUrl())
	configSecret.Data["config"] = currentConfigString
	configSecret.Data["name"] = []byte("ups")
	configSecret.Data["type"] = []byte("push")

	// Add the binding annotation to the UPS secret: this is done to link the actual ServiceBinding
	// Instance back to this secret. In case the variant is deleted in UPS we can use this ID to delete
	// the service binding
	bindingAnnotation := fmt.Sprintf("binding/%s", appType)
	if configSecret.Annotations == nil {
		configSecret.Annotations = make(map[string]string)
	}
	configSecret.Annotations[bindingAnnotation] = bindingId

	pushApplicationName, err := op.pushClient.getPushApplicationName()
	if err != nil {
		// don't fail because of name not fetched. just use the id as the name
		pushApplicationName = op.pushClient.getApplicationId()
	}

	log.Println("Adding annotations to mobile client")
	op.annotationHelper.addAnnotationToMobileClient(clientId, op.pushClient.getBaseUrl(), op.pushClient.getApplicationId(), pushApplicationName, appType, variantId, serviceInstanceName)

	_, err = op.kubeHelper.updateSecret(configSecret)
	if err != nil {
		log.Println(err.Error())
	} else {
		log.Printf("%s configuration of %s has been updated", appType, clientId)
	}
}
