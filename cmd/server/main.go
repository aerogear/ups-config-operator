package main

import (
	"os"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"math/rand"
	"time"

	mc "github.com/aerogear/mobile-crd-client/pkg/client/mobile/clientset/versioned"
	sc "github.com/aerogear/mobile-crd-client/pkg/client/servicecatalog/clientset/versioned"

	"github.com/aerogear/ups-config-operator/pkg/configOperator"
	"github.com/aerogear/ups-config-operator/pkg/constants"
)

func main() {
	rand.Seed(time.Now().Unix())

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	k8client := kubernetes.NewForConfigOrDie(config)
	scclient := sc.NewForConfigOrDie(config)
	mobileclient := mc.NewForConfigOrDie(config)

	pushClient, err := createPushClient(k8client)

	if err != nil {
		log.Printf("error initialising UPS client: %s", err.Error())
		return
	}

	annotationHelper := configOperator.NewAnnotationHelper(mobileclient)

	kubeHelper := configOperator.NewKubeHelper(k8client, scclient)

	operator := configOperator.NewConfigOperator(pushClient, annotationHelper, kubeHelper)

	operator.StartService()
}

func createPushClient(k8client *kubernetes.Clientset) (*configOperator.UpsClientImpl, error) {
	upsSecret, err := k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).Get(constants.UpsSecretName, metav1.GetOptions{})

	if err != nil {
		return &configOperator.UpsClientImpl{}, err
	}

	upsBaseURL := string(upsSecret.Data[constants.UpsSecretDataUrlKey])
	serviceInstanceId := upsSecret.Labels[constants.UpsSecretLabelServiceInstanceIdKey]

	config := &configOperator.PushApplication{
		ApplicationId: string(upsSecret.Data["applicationId"]),
	}

	pushClient := configOperator.NewUpsClientImpl(config, serviceInstanceId, upsBaseURL)

	return pushClient, nil
}
