package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type upsClient struct {
	config *pushApplication
}

func (client *upsClient) doesVariantWithNameExist(name string) bool {
	fmt.Println("Checking if a variant already exists with name", name)
	var url = "http://localhost:8080/rest/applications/" + client.config.ApplicationId + "/android"

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err.Error())
		return true
	} else {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)

		variants := make([]androidVariant, 0)
		json.Unmarshal(body, &variants)

		for _, variant := range variants {
			fmt.Println("Found a variant with name", name)
			if variant.Name == name {
				return true
			}
		}

		fmt.Println("No variant exists with name", name)

		return false
	}
}

func (client *upsClient) createVariant(json []byte) {
	fmt.Println("Creating a new variant in UPS")
	var url = "http://localhost:8080/rest/applications/" + client.config.ApplicationId + "/android"

	fmt.Println("Sending", string(json), "to", url)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(json))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	httpClient := http.Client{}
	_, err := httpClient.Do(req)

	if err != nil {
		fmt.Println("UPS request error", err.Error())
	}
}
