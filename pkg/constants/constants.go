package constants

const (
	EnvVarKeyNamespace = "NAMESPACE"

	K8SecretEventTypeAdded   = "ADDED"
	K8SecretEventTypeDeleted = "DELETED"

	// time in seconds
	UPSPollingInterval = 10

	UpsSecretName = "unified-push-server"

	UpsSecretDataUrlKey                = "uri"
	UpsSecretLabelServiceInstanceIdKey = "serviceInstanceID"

	SecretTypeLabelKey = "secretType"

	BindingSecretTypeMobile = "mobile-client-binding-secret"

	BindingDataServiceBindingIdKey    = "serviceBindingId"
	BindingDataServiceInstanceNameKey = "serviceInstanceName"

	BindingDataAppTypeKey  = "appType"
	BindingDataClientIdKey = "clientId"

	BindingDataGoogleKey        = "googleKey"
	BindingDataProjectNumberKey = "projectNumber"

	BindingDataIOSCertKey         = "cert"
	BindingDataIOSPassPhraseKey   = "passphrase"
	BindingDataIOSIsProductionKey = "isProduction"

	ExtAnnotationType = "unifiedpush"

	PushAppAnnotationNameFormat = "org.aerogear.binding.%s/push-application"
	UpsUrlAnnotationNameFormat = "org.aerogear.binding.%s/ups-url"
	ExtTypeAnnotationNameFormat = "org.aerogear.binding-ext-type.%s"
	ExtVariantsAnnotationNameFormat = "org.aerogear.binding-ext.%s/variants"
)
