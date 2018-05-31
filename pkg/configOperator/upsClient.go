package configOperator

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
)

type UpsClient interface {
	getPushApplicationName() (string, error)
	getVariants() ([]Variant, error)
	hasAndroidVariant(key string) *AndroidVariant
	createAndroidVariant(variant *AndroidVariant) (bool, *AndroidVariant)
	createIOSVariant(variant *IOSVariant) (bool, *IOSVariant)
	deleteVariant(platform string, variantId string) bool
	getApplicationId() string
	getServiceInstanceId() string
	getBaseUrl() string
}

type UpsClientImpl struct {
	config            *PushApplication
	serviceInstanceId string
	baseUrl           string
}

func NewUpsClientImpl(config *PushApplication, serviceInstanceId string, baseUrl string) *UpsClientImpl {
	client := new(UpsClientImpl)

	client.config = config
	client.serviceInstanceId = serviceInstanceId
	client.baseUrl = baseUrl

	return client
}

const BaseUrl = "http://localhost:8080/rest/applications"

// fetches the push application name from the UPS system
func (client *UpsClientImpl) getPushApplicationName() (string, error) {
	url := fmt.Sprintf("%s/%s", BaseUrl, client.config.ApplicationId)
	log.Printf("UPS request", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var pushAppInfo map[string]json.RawMessage
	err = json.Unmarshal(body, &pushAppInfo)
	if err != nil {
		return "", err
	}

	appNameWithQuotes := string(pushAppInfo["name"])            // --> e.g. "foo"
	appName := appNameWithQuotes[1 : len(appNameWithQuotes)-1]	// strip the quotes --> foo

	return appName, nil
}

func (client *UpsClientImpl) deleteVariant(platform string, variantId string) bool {
	variant := client.hasVariant(platform, variantId)

	if variant != nil {
		log.Printf("Deleting %s variant with id `%s`", platform, variant.VariantID)

		url := fmt.Sprintf("%s/%s/%s/%s", BaseUrl, client.config.ApplicationId,
			platform, variant.VariantID)

		log.Printf("UPS request", url)

		req, err := http.NewRequest(http.MethodDelete, url, nil)

		httpClient := http.Client{}
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Fatal(err.Error())
			return false
		}

		log.Printf("Variant `%s` has been deleted (status code %d)", variant.VariantID, resp.StatusCode)
		return resp.StatusCode == 204
	}

	log.Printf("No variant found to delete (Variant Id: `%s`)", variantId)
	return false
}

// Find an Android Variant by its Google Key
func (client *UpsClientImpl) hasAndroidVariant(key string) *AndroidVariant {
	variants, err := client.getAndroidVariants()

	if err != nil {
		log.Fatal(err)

		// Return true here to prevent creating a new variant when the
		// request fails
		return &AndroidVariant{}
	}

	for _, variant := range variants {
		if variant.GoogleKey == key {
			return &variant
		}
	}

	return nil
}

func (client *UpsClientImpl) createAndroidVariant(variant *AndroidVariant) (bool, *AndroidVariant) {
	url := fmt.Sprintf("%s/%s/android", BaseUrl, client.config.ApplicationId)
	log.Printf("UPS request", url)

	payload, err := json.Marshal(variant)
	if err != nil {
		panic(err.Error())
	}

	log.Println("UPS Payload", string(payload))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	httpClient := http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		panic(err.Error())
	}

	log.Printf("UPS responded with status code ", resp.StatusCode)

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var createdVariant AndroidVariant
	json.Unmarshal(body, &createdVariant)

	return resp.StatusCode == 201, &createdVariant
}

func (client *UpsClientImpl) createIOSVariant(variant *IOSVariant) (bool, *IOSVariant) {
	url := fmt.Sprintf("%s/%s/ios", BaseUrl, client.config.ApplicationId)
	log.Printf("UPS request", url)

	production := "true"
	if !variant.Production {
		production = "false"
	}

	params := map[string]string{
		"name":        variant.Name,
		"passphrase":  variant.Passphrase,
		"production":  production,
		"description": variant.Description,
	}

	// We need to decode it before sending
	decodedString, err := base64.StdEncoding.DecodeString(string(variant.Certificate))
	if err != nil {
		log.Print("Invalid cert - Please check this cert is in base64 encoded format: ", err)
	}

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("certificate", "certificate")
	if err != nil {
		panic(err.Error())
	}
	part.Write(decodedString)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}

	defer writer.Close()

	req, err := http.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	httpClient := http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		panic(err.Error())
	}

	log.Printf("UPS responded with status code: %s ", resp.StatusCode)

	defer resp.Body.Close()
	b, _ := ioutil.ReadAll(resp.Body)
	var createdVariant IOSVariant
	json.Unmarshal(b, &createdVariant)
	return resp.StatusCode == 201, &createdVariant
}

func (client *UpsClientImpl) getVariantsForPlatform(platform string) ([]Variant, error) {
	variantBytes, err := client.getVariantsForPlatformRaw(platform)

	if err != nil {
		return nil, err
	}

	variants := make([]Variant, 0)
	json.Unmarshal(variantBytes, &variants)

	return variants, nil
}

func (client *UpsClientImpl) getVariants() ([]Variant, error) {

	UPSIOSVariants, err := client.getVariantsForPlatform("ios")
	UPSAndroidVariants, err := client.getVariantsForPlatform("android")

	if err != nil {
		return nil, err
	}

	variants := append(UPSAndroidVariants, UPSIOSVariants...)

	return variants, nil
}

func (client *UpsClientImpl) getApplicationId() string {
	return client.config.ApplicationId
}

func (client *UpsClientImpl) getServiceInstanceId() string {
	return client.serviceInstanceId
}

func (client *UpsClientImpl) getBaseUrl() string {
	return client.baseUrl
}

////////////////////////////////////// internal things /////////////////////////////////////

func (client *UpsClientImpl) getAndroidVariants() ([]AndroidVariant, error) {
	variantsBytes, err := client.getVariantsForPlatformRaw("android")
	if err != nil {
		return nil, err
	}
	androidVariants := make([]AndroidVariant, 0)
	json.Unmarshal(variantsBytes, &androidVariants)
	return androidVariants, nil
}

// Find a Variant by its variant id
func (client *UpsClientImpl) hasVariant(platform string, variantId string) *Variant {
	variants, err := client.getVariantsForPlatform(platform)

	if err != nil {
		log.Fatal(err)

		// Return true here to prevent creating a new variant when the
		// request fails
		return &Variant{}
	}

	for _, variant := range variants {
		if variant.VariantID == variantId {
			return &variant
		}
	}

	return nil
}

func (client *UpsClientImpl) getVariantsForPlatformRaw(platform string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s", BaseUrl, client.config.ApplicationId, platform)
	log.Printf("UPS request", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return body, nil
}
