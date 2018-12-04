package main

import (
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"math/rand"
	"time"

	mc "github.com/aerogear/mobile-crd-client/pkg/client/mobile/clientset/versioned"
	sc "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"

	"github.com/aerogear/ups-config-operator/pkg/configOperator"
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
	pushClientProvider := configOperator.NewUpsClientProviderImpl(k8client)

	if err != nil {
		log.Printf("error initialising UPS client: %s", err.Error())
		return
	}

	annotationHelper := configOperator.NewAnnotationHelper(mobileclient)

	kubeHelper := configOperator.NewKubeHelper(k8client, scclient)

	operator := configOperator.NewConfigOperator(pushClientProvider, annotationHelper, kubeHelper)

	// This is blocking. Any code after this will not be called
	operator.StartService()
}
