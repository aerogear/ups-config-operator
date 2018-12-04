package configOperator

import (
	"bytes"
	"encoding/json"
	"k8s.io/api/core/v1"

	"github.com/pkg/errors"
)

// This is required because importing core/v1/Secret leads to a double import and redefinition
// of log_dir
type BindingSecret = v1.Secret

type Variant struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VariantID   string `json:"variantID"`
	Secret      string `json:"secret"`
}

type AndroidVariant struct {
	ProjectNumber string `json:"projectNumber"`
	GoogleKey     string `json:"googleKey"`
	Variant
}

type IOSVariant struct {
	Certificate []byte `json:"certificate"`
	Passphrase  string `json:"passphrase"`
	Production  bool   `json:"production"`
	Variant
}

type PushApplication struct {
	ApplicationId string `json:"applicationId"`
}

func (this *AndroidVariant) getJson() ([]byte, error) {
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

func (this *IOSVariant) getJson() ([]byte, error) {
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
	Android *map[string]string `json:"android,omitempty"`
	IOS     *map[string]string `json:"ios,omitempty"`
}

type VariantServiceBindingMapping struct {
	VariantId        string
	ServiceBindingId string
}

func GetClientConfigRepresentation(variantId, serviceBindingId string) (VariantServiceBindingMapping, error) {
	config := VariantServiceBindingMapping{
		VariantId:        variantId,
		ServiceBindingId: serviceBindingId,
	}
	return config, config.Validate()
}

func (configRepresentation *VariantServiceBindingMapping) Validate() error {
	if configRepresentation.VariantId == "" {
		return errors.New("missing variantId")
	} else if configRepresentation.ServiceBindingId == "" {
		return errors.New("missing serviceBindingId")
	}
	return nil
}
