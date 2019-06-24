package configOperator

import (
	"os"
	"github.com/aerogear/ups-config-operator/pkg/constants"
	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
)

// Provides ups clients.
// This is to allow creation of 
type UpsClientProvider interface {
	getPushClient() UpsClient
}

type UpsClientProviderImpl struct {
	k8client         *kubernetes.Clientset
	cachedPushClient *UpsClientImpl
}

func NewUpsClientProviderImpl(k8client *kubernetes.Clientset) *UpsClientProviderImpl {
	provider := new(UpsClientProviderImpl)
	provider.k8client = k8client;
	return provider
}

func (p *UpsClientProviderImpl) getPushClient() UpsClient {
	if p.cachedPushClient == nil {
		client, err := createPushClient(p.k8client)

		if err != nil {
			log.Printf("Error creating push client: %v", err.Error())
		}

		p.cachedPushClient = client
	}

	return p.cachedPushClient
}

func createPushClient(k8client *kubernetes.Clientset) (*UpsClientImpl, error) {
	upsSecret, err := k8client.CoreV1().Secrets(os.Getenv(constants.EnvVarKeyNamespace)).Get(constants.UpsSecretName, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	upsBaseURL := string(upsSecret.Data[constants.UpsSecretDataUrlKey])
	serviceInstanceId := upsSecret.Labels[constants.UpsSecretLabelServiceInstanceIdKey]

	config := &PushApplication{
		ApplicationId: string(upsSecret.Data["applicationId"]),
	}

	pushClient := NewUpsClientImpl(config, serviceInstanceId, upsBaseURL)

	return pushClient, nil
}
