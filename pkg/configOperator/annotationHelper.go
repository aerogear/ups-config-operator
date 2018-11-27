package configOperator

import (
	"fmt"
	"os"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mc "github.com/aerogear/mobile-crd-client/pkg/client/mobile/clientset/versioned"
	"github.com/aerogear/ups-config-operator/pkg/constants"
	"encoding/json"
	"strings"
)

type AnnotationHelper interface {
	addAnnotationToMobileClient(clientId string, upsUrl string, pushApplicationId string, pushApplicationName string, appType string, variantUrl string, serviceInstanceName string)
	removeAnnotationFromMobileClient(clientId string, appType string, serviceInstanceName string)
}

type AnnotationHelperImpl struct {
	mobileclient *mc.Clientset
}

func NewAnnotationHelper(mobileclient *mc.Clientset) *AnnotationHelperImpl {
	helper := new(AnnotationHelperImpl)

	helper.mobileclient = mobileclient

	return helper
}

// Adds an annotation to the mobile client that contains information about this variant
// (currently URL and Name)
func (helper AnnotationHelperImpl) addAnnotationToMobileClient(clientId string, upsUrl string, pushApplicationId string, pushApplicationName string, appType string, variantId string, serviceInstanceName string) {
	client, err := helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Get(clientId, metav1.GetOptions{})
	if err != nil {
		log.Printf("No mobile client with name %s found", clientId)
		return
	}

	pushApplicationUrl := upsUrl + "/#/app/" + pushApplicationId + "/variants"
	variantUrl := pushApplicationUrl + "/" + variantId

	pushAppAnnotationName := fmt.Sprintf(constants.PushAppAnnotationNameFormat, serviceInstanceName)
	pushAppAnnotationValue := fmt.Sprintf(`{"label":"Push Application","type":"href","text":"%s", "value":"%s"}`, pushApplicationName, pushApplicationUrl)

	upsUrlAnnotationName := fmt.Sprintf(constants.UpsUrlAnnotationNameFormat, serviceInstanceName)
	upsUrlAnnotationValue := fmt.Sprintf(`{"label":"UPS Admin Console URL","type":"href","value":"%s"}`, upsUrl)

	extVariantAnnotationName := fmt.Sprintf(constants.ExtVariantsAnnotationNameFormat, serviceInstanceName)
	extVariantAnnotationConfigForSingleVariant := variantAnnotationConfig{
		Type:      appType,
		TypeLabel: getVariantTypeLabel(appType),
		Url:       variantUrl,
		Id:        variantId,
	}
	var extVariantAnnotationConfigValue []variantAnnotationConfig

	if client.Annotations == nil {
		client.Annotations = make(map[string]string)
	}

	client.Annotations[pushAppAnnotationName] = pushAppAnnotationValue
	client.Annotations[upsUrlAnnotationName] = upsUrlAnnotationValue

	extVariantAnnotationConfigValue = []variantAnnotationConfig{
		extVariantAnnotationConfigForSingleVariant,
	}

	if client.Annotations[extVariantAnnotationName] != "" {
		var existingVariantConfigs []variantAnnotationConfig
		err = json.Unmarshal([]byte(client.Annotations[extVariantAnnotationName]), &existingVariantConfigs)

		if err != nil {
			log.Printf("Error unmarshalling variant annotation config for name %s. Error: %s", client.Annotations[extVariantAnnotationName], err.Error())
			return
		}

		for _, variantConfig := range existingVariantConfigs {
			if variantConfig.Type == appType {
				continue
			} else {
				extVariantAnnotationConfigValue = append(extVariantAnnotationConfigValue, variantConfig)
			}
		}
	}

	extVariantAnnotationConfigValueStr, err := json.Marshal(extVariantAnnotationConfigValue)

	if err != nil {
		log.Printf("Error marshalling newly built variant annotation config for name %s. Value: %s, Error: %s", extVariantAnnotationConfigValueStr, extVariantAnnotationConfigValue, err.Error())
		return
	}

	client.Annotations[extVariantAnnotationName] = string(extVariantAnnotationConfigValueStr)

	_, err = helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Update(client)
	if err != nil {
		log.Printf(err.Error())
	}
}

func (helper AnnotationHelperImpl) removeAnnotationFromMobileClient(clientId string, appType string, serviceInstanceName string) {
	client, err := helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Get(clientId, metav1.GetOptions{})
	if err != nil {
		log.Printf("No mobile client with name %s found", clientId)
		return
	}

	if client.Annotations == nil {
		log.Printf("Mobile client doesn't have any annotations. Returning w/o any operation")
		return
	}

	extVariantAnnotationName := fmt.Sprintf(constants.ExtVariantsAnnotationNameFormat, serviceInstanceName)

	var existingVariantConfigs []variantAnnotationConfig
	err = json.Unmarshal([]byte(client.Annotations[extVariantAnnotationName]), &existingVariantConfigs)

	if err != nil {
		log.Printf("Error unmarshalling variant annotation config for name %s. Error: %s", client.Annotations[extVariantAnnotationName], err.Error())
		return
	}

	var extVariantAnnotationConfigValue = []variantAnnotationConfig{}

	for _, variantConfig := range existingVariantConfigs {
		if variantConfig.Type == appType {
			continue
		} else {
			extVariantAnnotationConfigValue = append(extVariantAnnotationConfigValue, variantConfig)
		}
	}

	if len(extVariantAnnotationConfigValue) > 0 {
		newConfigStr, err := json.Marshal(extVariantAnnotationConfigValue)

		if err != nil {
			log.Printf("Error marshalling newly built variant annotation config for name %s. Value: %s, Error: %s", client.Annotations[extVariantAnnotationName], extVariantAnnotationConfigValue, err.Error())
			return
		}

		client.Annotations[extVariantAnnotationName] = string(newConfigStr)

		_, err = helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Update(client)
		if err != nil {
			log.Printf("Unable to update mobile client %s. Error: %s", clientId, err.Error())
		}

	} else {
		log.Println("Removing all push related annotations from the mobile client as there are no variants anymore")

		pushAppAnnotationName := fmt.Sprintf(constants.PushAppAnnotationNameFormat, serviceInstanceName)
		upsUrlAnnotationName := fmt.Sprintf(constants.UpsUrlAnnotationNameFormat, serviceInstanceName)

		delete(client.Annotations, pushAppAnnotationName)
		delete(client.Annotations, upsUrlAnnotationName)
		delete(client.Annotations, extVariantAnnotationName)

		_, err = helper.mobileclient.MobileV1alpha1().MobileClients(os.Getenv(constants.EnvVarKeyNamespace)).Update(client)
		if err != nil {
			log.Printf("Unable to update mobile client %s. Error: %s", clientId, err.Error())
		}
	}

}

type variantAnnotationConfig struct {
	Type      string `json:"type"`
	TypeLabel string `json:"typeLabel"`
	Url       string `json:"url"`
	Id        string `json:"id"`
}

func getVariantTypeLabel(variantType string) string {
	if strings.EqualFold(variantType, "android") {
		return "Android"
	} else if strings.EqualFold(variantType, "ios") {
		return "iOS"
	} else {
		return variantType
	}
}
