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
	Cert []byte `json:"cert"`
	PassPhrase string `json:"pass_phrase"`
	variant
}

type pushApplication struct {
	ApplicationId string `json:"applicationId"`
}
