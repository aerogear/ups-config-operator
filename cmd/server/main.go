package main

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"

	"fmt"

	"github.com/prometheus/common/log"
	"github.com/satori/go.uuid"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	mobile "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
)

var k8client *kubernetes.Clientset
var pushClient *upsClient

const NamespaceKey = "NAMESPACE"
const ActionAdded = "ADDED"
const SecretTypeKey = "secretType"
const BindingSecretType = "mobile-client-binding-secret"
const BindingAppType = "appType"

const BindingGoogleKey = "googleKey"
const BindingVariantName = "variantName"
const BindingProjectNumber = "projectNumber"

const UpsSecretName = "unified-push-server"

// This is required because importing core/v1/Secret leads to a double import and redefinition
// of log_dir
type BindingSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Data              map[string][]byte `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`
	StringData        map[string]string `json:"stringData,omitempty" protobuf:"bytes,4,rep,name=stringData"`
}

// Deletes the binding secret after the sync operation has completed
func deleteSecret(name string) {
	err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Delete(name, nil)
	if err != nil {
		log.Error("Error deleting bind secret", err)
	}

	log.Info(fmt.Sprintf("Secret `%s` has been deleted", name))
}

func handleAndroidVariant(key string, name string, pn string) {
	// Only instantiate the push client here because we need to wait for the ups secret to
	// be available
	if pushClient == nil {
		pushClient = pushClientOrDie()
	}

	if pushClient.hasAndroidVariant(key) == false {
		payload := &androidVariant{
			ProjectNumber: pn,
			GoogleKey:     key,
			variant: variant{
				Name:      name,
				VariantID: uuid.NewV4().String(),
				Secret:    uuid.NewV4().String(),
			},
		}

		log.Info("Creating a new android variant", payload)
		pushClient.createAndroidVariant(payload)
	} else {
		log.Info(fmt.Sprint("A variant for google key '%s' already exists", key))
	}
}

func handleAddSecret(obj runtime.Object) {
	raw, _ := json.Marshal(obj)
	var secret = BindingSecret{}
	json.Unmarshal(raw, &secret)

	if val, ok := secret.Labels[SecretTypeKey]; ok && val == BindingSecretType {
		appType := string(secret.Data[BindingAppType])

		if appType == "Android" {
			log.Info("A mobile binding secret of type `Android` was added")
			googleKey := string(secret.Data[BindingGoogleKey])
			variantName := string(secret.Data[BindingVariantName])
			projectNumber := string(secret.Data[BindingProjectNumber])
			handleAndroidVariant(googleKey, variantName, projectNumber)
		}

		// Always delete the secret after handling it regardless of any new resources
		// was created
		deleteSecret(secret.Name)
	}
}

func watchLoop() {
	events, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Watch(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}

	for update := range events.ResultChan() {
		switch action := update.Type; action {
		case ActionAdded:
			handleAddSecret(update.Object)
		default:
			log.Info("Unhandled action:", action)
		}
	}
}

func convertSecretToUpsSecret(s *mobile.Secret) *pushApplication {
	return &pushApplication{
		ApplicationId: string(s.Data["applicationId"]),
	}
}

func kubeOrDie(config *rest.Config) *kubernetes.Clientset {
	k8client, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return k8client
}

func pushClientOrDie() *upsClient {
	upsSecret, err := k8client.CoreV1().Secrets(os.Getenv(NamespaceKey)).Get(UpsSecretName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	return &upsClient{
		config: convertSecretToUpsSecret(upsSecret),
	}
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	k8client = kubeOrDie(config)

	log.Info("Entering watch loop")

	for {
		watchLoop()
	}
}
