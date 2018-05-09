package main

import (
	"bytes"
	"encoding/json"
)

type variant struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VariantID   string `json:"variantID"`
	Secret      string `json:"secret"`
}

type androidVariant struct {
	ProjectNumber string `json:"projectNumber"`
	GoogleKey     string `json:"googleKey"`
	variant
}

type iOSVariant struct {
	Certificate []byte `json:"certificate"`
	Passphrase  string `json:"passphrase"`
	Production  bool   `json:"production"`
	variant
}

type pushApplication struct {
	ApplicationId string `json:"applicationId"`
}

type VariantAnnotation struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

func (this *androidVariant) getJson() ([]byte, error) {
	config := map[string]string{
		"senderId":      this.ProjectNumber,
		"variantId":     this.VariantID,
		"variantSecret": this.Secret,
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(config)
	return buffer.Bytes(), err
}

func (this *iOSVariant) getJson() ([]byte, error) {
	config := map[string]string{
		"variantId":     this.VariantID,
		"variantSecret": this.Secret,
	}

	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(config)
	return buffer.Bytes(), err
}

type UPSClientConfig struct {
	Android       *map[string]string `json:"android,omitempty"`
	IOS           *map[string]string `json:"ios,omitempty"`
	PushServerURL string             `json:"pushServerUrl,omitempty"`
}
