package main

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

type upsClient struct {
	config            *pushApplication
	serviceInstanceId string
	baseUrl           string
}

const BaseUrl = "http://localhost:8080/rest/applications"

func (client *upsClient) deleteVariant(platform string, variantId string) bool {
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

// Find a Variant by its variant id
func (client *upsClient) hasVariant(platform string, variantId string) *variant {
	variants, err := client.getVariantsForPlatform(platform)

	if err != nil {
		log.Fatal(err)

		// Return true here to prevent creating a new variant when the
		// request fails
		return &variant{}
	}

	for _, variant := range variants {
		if variant.VariantID == variantId {
			return &variant
		}
	}

	return nil
}

// Find an Android Variant by its Google Key
func (client *upsClient) hasAndroidVariant(key string) *androidVariant {
	variants, err := client.getAndroidVariants()

	if err != nil {
		log.Fatal(err)

		// Return true here to prevent creating a new variant when the
		// request fails
		return &androidVariant{}
	}

	for _, variant := range variants {
		if variant.GoogleKey == key {
			return &variant
		}
	}

	return nil
}

func (client *upsClient) createAndroidVariant(variant *androidVariant) (bool, *androidVariant) {
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
	var createdVariant androidVariant
	json.Unmarshal(body, &createdVariant)

	return resp.StatusCode == 201, &createdVariant
}

func (client *upsClient) createIOSVariant(variant *iOSVariant) (bool, *iOSVariant) {
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
	var createdVariant iOSVariant
	json.Unmarshal(b, &createdVariant)
	return resp.StatusCode == 201, &createdVariant
}

func (client *upsClient) getVariantsForPlatformRaw(platform string) ([]byte, error) {
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

func (client *upsClient) getVariantsForPlatform(platform string) ([]variant, error) {
	variantBytes, err := client.getVariantsForPlatformRaw(platform)

	if err != nil {
		return nil, err
	}

	variants := make([]variant, 0)
	json.Unmarshal(variantBytes, &variants)

	return variants, nil
}

func (client *upsClient) getAndroidVariants() ([]androidVariant, error) {
	variantsBytes, err := client.getVariantsForPlatformRaw("android")
	if err != nil {
		return nil, err
	}
	androidVariants := make([]androidVariant, 0)
	json.Unmarshal(variantsBytes, &androidVariants)
	return androidVariants, nil
}

func (client *upsClient) getIOSVariants() ([]iOSVariant, error) {
	variantsBytes, err := client.getVariantsForPlatformRaw("ios")
	if err != nil {
		return nil, err
	}
	iOSVariants := make([]iOSVariant, 0)
	json.Unmarshal(variantsBytes, &iOSVariants)
	return iOSVariants, nil
}

func (client *upsClient) getVariants() ([]variant, error) {

	UPSIOSVariants, err := client.getVariantsForPlatform("ios")
	UPSAndroidVariants, err := client.getVariantsForPlatform("android")

	if err != nil {
		return nil, err
	}

	variants := append(UPSAndroidVariants, UPSIOSVariants...)

	return variants, nil
}
