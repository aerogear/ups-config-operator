package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"mime/multipart"
	"os"
)

type upsClient struct {
	config *pushApplication
	serviceInstanceId string
	baseUrl string
}

const BaseUrl = "http://localhost:8080/rest/applications"

// Delete the variant with the given google key
func (client *upsClient) deleteVariant(key string) bool {
	variant := client.hasAndroidVariant(key)
	if variant != nil {
		log.Printf("Deleting variant with id `%s`", variant.VariantID)

		url := fmt.Sprintf("%s/%s/adm/%s", BaseUrl, client.config.ApplicationId, variant.VariantID)
		log.Printf("UPS request", url)

		req, err := http.NewRequest(http.MethodDelete, url, nil)

		httpClient := http.Client{}
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Fatal(err.Error())
			return false
		}

		log.Printf("Variant `%s` has been deleted", variant.VariantID)
		return resp.StatusCode == 204
	}

	log.Printf("No variant found to delete (google key: `%s`)", key)
	return false
}

// Find an Android Variant by it's Google Key
func (client *upsClient) hasAndroidVariant(key string) *androidVariant {
	url := fmt.Sprintf("%s/%s/android", BaseUrl, client.config.ApplicationId)
	log.Printf("UPS request", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)

		// Return true here to prevent creating a new variant when the
		// request fails
		return &androidVariant{}
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	variants := make([]androidVariant, 0)
	json.Unmarshal(body, &variants)

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

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	httpClient := http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		panic(err.Error())
	}

	log.Printf("UPS responded with status code ", resp.Status)

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var createdVariant androidVariant
	json.Unmarshal(body, &createdVariant)

	return resp.StatusCode == 201, &createdVariant
}

func (client *upsClient) createIOSVariant(variant *iOSVariant) (bool, *iOSVariant) {
	url := fmt.Sprintf("%s/%s/ios", BaseUrl, client.config.ApplicationId)

//	log.Printf("UPS request:  %s", url)

	params :=  map[string]string{
		"name": variant.Name,
		"passPhrase": variant.PassPhrase,
		"production" : "false",
		"description": variant.Description,
	}

	mode := int(0777)
	perms:= os.FileMode(mode)

	err := ioutil.WriteFile("/tmp/certFile", variant.Certificate, perms)

	if err != nil {
		panic(err)
	}


	file, err := os.Open("/tmp/certFile")
	if err != nil {
		log.Print("err:", err)
	}

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		log.Print("err:", err)
	}

	fi, err := file.Stat()
	if err != nil {
		log.Print("err:", err)
	}

	defer file.Close()

	payload := new(bytes.Buffer)
	writer := multipart.NewWriter(payload)

	part, err := writer.CreateFormFile("certificate", fi.Name())
	part.Write(fileContents)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}

	writer.Close()

	req, err := http.NewRequest(http.MethodPost, url, payload)
	log.Print(req.MultipartForm.Value)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	log.Printf("UPS REQUEST: " , req)

	req.Header.Set("Accept", "application/json")

	httpClient := http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		panic(err.Error())
	}

	log.Printf("UPS responded with status code: %s ", resp.Status)

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	var createdVariant iOSVariant
	json.Unmarshal(body, &createdVariant)

	return resp.StatusCode == 201, &createdVariant
}