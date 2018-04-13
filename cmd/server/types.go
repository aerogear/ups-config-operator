package main

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
	PassPhrase string `json:"passPhrase"`
	variant
}

type pushApplication struct {
	ApplicationId string `json:"applicationId"`
}
