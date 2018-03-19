package main

type upsVariant struct {
	ProjectNumber string `json:"projectNumber"`
	GoogleKey     string `json:"googleKey"`
	Name          string `json:"name"`
}

type upsAppData struct {
	ApplicationId string `json:"applicationId"`
}
