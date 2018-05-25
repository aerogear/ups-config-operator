package configOperator

import (
	"fmt"
	"os"
	"encoding/json"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mc "github.com/aerogear/mobile-crd-client/pkg/client/mobile/clientset/versioned"
	"github.com/aerogear/ups-config-operator/pkg/constants"
)

type AnnotationHelper struct {
	mobileclient *mc.Clientset
}

func NewAnnotationHelper(mobileclient *mc.Clientset) *AnnotationHelper {
	helper := new(AnnotationHelper)

	helper.mobileclient = mobileclient

	return helper
}

// Creates the JSON string for the mobile-client variant annotation
func (helper AnnotationHelper) generateVariantAnnotationValue(url string, appType string) ([]byte, error) {
	annotation := VariantAnnotation{
		Type:  "href",
		Label: fmt.Sprintf("UPS %s Variant", appType),
		Value: url,
	}

	return json.Marshal(annotation)
}

// Adds an annotation to the mobile client that contains information about this variant
// (currently URL and Name)
func (helper AnnotationHelper) AddAnnotationToMobileClient(clientId string, appType string, variantUrl string, serviceInstanceName string) {
	client, err := helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Get(clientId, metav1.GetOptions{})
	if err != nil {
		log.Printf("No mobile client with name %s found", clientId)
		return
	}

	annotationName := fmt.Sprintf("org.aerogear.binding.%s/variant-%s", serviceInstanceName, appType)
	annotationValue, err := helper.generateVariantAnnotationValue(variantUrl, appType)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	if client.Annotations == nil {
		client.Annotations = make(map[string]string)
	}

	client.Annotations[annotationName] = string(annotationValue)
	_, err = helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Update(client)
	if err != nil {
		log.Printf(err.Error())
	}
}

func (helper AnnotationHelper) RemoveAnnotationFromMobileClient(clientId string, appType string, serviceInstanceName string) {
	client, err := helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Get(clientId, metav1.GetOptions{})
	if err != nil {
		log.Printf("No mobile client with name %s found", clientId)
		return
	}

	if client.Annotations != nil {
		annotationName := fmt.Sprintf("org.aerogear.binding.%s/variant-%s", serviceInstanceName, appType)
		log.Printf("Removing annotation %s from mobile client %s", annotationName, clientId)

		delete(client.Annotations, annotationName)
		_, err = helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Update(client)
		if err != nil {
			log.Printf(err.Error())
		}
	}
}
