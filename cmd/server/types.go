package main

type variant struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	VariantID     string `json:"variantId"`
	Secret        string `json:"secret"`
}
type androidVariant struct {
	ProjectNumber string `json:"projectNumber"`
	GoogleKey     string `json:"googleKey"`
	variant
}

type pushApplication struct {
	ApplicationId string `json:"applicationId"`
}
