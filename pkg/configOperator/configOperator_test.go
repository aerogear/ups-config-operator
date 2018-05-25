package configOperator

import (
	"testing"

	"k8s.io/client-go/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/stretchr/testify/mock"
	"reflect"
	"github.com/stretchr/testify/assert"
)

var op *ConfigOperator;

var pushClient *MockUpsClient
var annotationHelper *MockAnnotationHelper
var kubeHelper *MockKubeHelper

func setup() {
	pushClient = new(MockUpsClient)
	annotationHelper = new(MockAnnotationHelper)
	kubeHelper = new(MockKubeHelper)

	op = NewConfigOperator(pushClient, annotationHelper, kubeHelper)
}

func TestConfigOperator_compareUPSVariantsWithClientConfigs(t *testing.T) {
	setup()

	// create secret list
	secretData1 := map[string][]byte{
		"config": []byte("{\"Android\":{\"variantId\":\"foo\"}}"),
	}
	objectMeta1 := metav1.ObjectMeta{
		Annotations: map[string]string{
			"binding/android": "toBeKept",
		},
	}

	secretData2 := map[string][]byte{
		"config": []byte("{\"IOS\":{\"variantId\":\"bar\"}}"),
	}
	objectMeta2 := metav1.ObjectMeta{
		Annotations: map[string]string{
			"binding/ios": "toBeDeleted",
		},
	}

	secretList := &v1.SecretList{Items: []v1.Secret{
		{Data: secretData1, ObjectMeta: objectMeta1},
		{Data: secretData2, ObjectMeta: objectMeta2}},
	}

	// create variant list
	variantList := []Variant{
		{VariantID: "foo"},
	}

	pushClient.On("getApplicationId").Return("myapp")
	kubeHelper.On("listSecrets", "serviceName=ups,pushApplicationId=myapp").Return(secretList, nil)
	pushClient.On("getVariants").Return(variantList, nil)
	kubeHelper.On("getServiceBindingNameByID", "toBeDeleted").Return("nameOfTheServiceBindingToDelete", nil)
	kubeHelper.On("deleteServiceBinding", "nameOfTheServiceBindingToDelete").Return(nil)

	op.compareUPSVariantsWithClientConfigs()

	kubeHelper.AssertExpectations(t)
}

func TestConfigOperator_handleDeleteSecret_whenThereAre2Variants(t *testing.T) {
	setup()

	bindingSecret := BindingSecret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ServiceBinding"},
				{Kind: "SomethingElse"},
			},
		},
		Data: map[string][]byte{
			"appType":  []byte("ANdroId"),
			"clientId": []byte("myClientId"),
		},
	}

	configSecret := &v1.Secret{
		Data: map[string][]byte{
			"serviceInstanceName": []byte("myServiceInstanceName"),
			"config":              []byte("{\"android\":{\"variantId\":\"myVariantId\", \"foo\":\"bar\"}, \"ios\":{\"variantId\":\"yourVariantId\",\"pop\":\"cake\"}}"),
		},
	}

	configSecret.Annotations = map[string]string{
		"binding/android": "toBeGone",
		"binding/ios":     "toBeKept",
	}

	kubeHelper.On("findMobileClientConfig", "myClientId").Return(configSecret)
	annotationHelper.On("removeAnnotationFromMobileClient", "myClientId", "android", "myServiceInstanceName").Once()
	kubeHelper.On("updateSecret", mock.Anything).Return(nil, nil)
	pushClient.On("deleteVariant", "android", "myVariantId").Return(true)

	op.handleDeleteSecret(&bindingSecret)

	kubeHelper.AssertCalled(t, "updateSecret", mock.MatchedBy(func(secret *v1.Secret) bool {
		// Annotation for Android should be deleted
		annotationGood := reflect.DeepEqual(secret.Annotations, map[string]string{
			"binding/ios": "toBeKept",
		})

		// config for Android should be deleted
		secretConfigGood := string(secret.Data["config"]) == "{\"ios\":{\"variantId\":\"yourVariantId\",\"pop\":\"cake\"}}"

		return annotationGood && secretConfigGood
	}))

	kubeHelper.AssertNotCalled(t, "deleteSecret", mock.Anything)
}

func TestConfigOperator_handleDeleteSecret_whenThereIs1Variant(t *testing.T) {
	setup()

	bindingSecret := BindingSecret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ServiceBinding"},
				{Kind: "SomethingElse"},
			},
		},
		Data: map[string][]byte{
			"appType":  []byte("ANdroId"),
			"clientId": []byte("myClientId"),
		},
	}

	configSecret := &v1.Secret{
		Data: map[string][]byte{
			"serviceInstanceName": []byte("myServiceInstanceName"),
			"config":              []byte("{\"android\":{\"variantId\":\"myVariantId\", \"foo\":\"bar\"}}"),
		},
	}

	configSecret.Name = "mySecretName"

	configSecret.Annotations = map[string]string{
		"binding/android": "toBeGone",
	}

	kubeHelper.On("findMobileClientConfig", "myClientId").Return(configSecret)
	annotationHelper.On("removeAnnotationFromMobileClient", "myClientId", "android", "myServiceInstanceName").Once()
	kubeHelper.On("deleteSecret", "mySecretName").Once()
	pushClient.On("deleteVariant", "android", "myVariantId").Return(true)

	op.handleDeleteSecret(&bindingSecret)

	kubeHelper.AssertCalled(t, "deleteSecret", "mySecretName")
	kubeHelper.AssertNotCalled(t, "updateSecret", mock.Anything)
}

func TestConfigOperator_handleAddSecret_whenAndroid_andAVariantExistsWithSameGoogleKey(t *testing.T) {
	setup()

	bindingSecret := BindingSecret{
		Data: map[string][]byte{
			"appType":   []byte("Android"),
			"googleKey": []byte("myGoogleKey"),
		},
	}
	bindingSecret.Labels = map[string]string{
		"secretType": "mobile-client-binding-secret",
	}
	bindingSecret.Name = "myBindingSecret"

	pushClient.On("hasAndroidVariant", "myGoogleKey").Return(&AndroidVariant{
		GoogleKey: "myGoogleKey",
	})

	kubeHelper.On("deleteSecret", "myBindingSecret").Once()

	op.handleAddSecret(&bindingSecret)

	assert.Empty(t, annotationHelper.Calls)

	kubeHelper.AssertNotCalled(t, "createClientConfigSecret", mock.Anything)
	kubeHelper.AssertNotCalled(t, "updateSecret", mock.Anything)

	kubeHelper.AssertExpectations(t)
	annotationHelper.AssertExpectations(t)
}

func TestConfigOperator_handleAddSecret_whenAndroid_andNoVariantExistsWithSameGoogleKey(t *testing.T) {
	setup()

	bindingSecret := BindingSecret{
		Data: map[string][]byte{
			"appType":             []byte("Android"),
			"clientId":            []byte("myClientId"),
			"googleKey":           []byte("myGoogleKey"),
			"projectNumber":       []byte("myProjectNumber"),
			"serviceBindingId":    []byte("myServiceBindingId"),
			"serviceInstanceName": []byte("myServiceInstanceName"),
		},
	}
	bindingSecret.Labels = map[string]string{
		"secretType": "mobile-client-binding-secret",
	}
	bindingSecret.Name = "myBindingSecret"

	pushClient.On("getServiceInstanceId").Return("myPushServiceInstanceId")
	pushClient.On("getApplicationId").Return("myPushApplicationId")
	pushClient.On("getBaseUrl").Return("http://example.org")
	pushClient.On("hasAndroidVariant", "myGoogleKey").Return(nil)
	pushClient.On("createAndroidVariant", mock.Anything).Return(true, &AndroidVariant{
		ProjectNumber: "myProjectNumber",
		GoogleKey:     "myGoogleKey",
		Variant: Variant{
			Name:      "myAndroidVariant",
			VariantID: "myVariantId",
			Secret:    "myVariantSecret",
		},
	})

	// no existing client config
	kubeHelper.On("findMobileClientConfig", "myClientId").Return(nil)

	configSecret := &v1.Secret{
		Data: map[string][]byte{
			"serviceInstanceName": []byte("myServiceInstanceName"),
			"config":              []byte("{\"ios\":{\"variantId\":\"yourVariantId\",\"pop\":\"cake\"}}"),
		},
	}
	configSecret.Name = "mySecretName"
	configSecret.Annotations = map[string]string{
		"binding/ios": "toBeKept",
	}

	kubeHelper.On("createClientConfigSecret", "myClientId", "myServiceInstanceName", "myPushServiceInstanceId", "myPushApplicationId").Return(configSecret)
	annotationHelper.On("addAnnotationToMobileClient", "myClientId", "android", "http://example.org/#/app/myPushApplicationId/variants/myVariantId", "myServiceInstanceName").Once()
	kubeHelper.On("updateSecret", mock.Anything).Return(nil, nil)
	kubeHelper.On("deleteSecret", "myBindingSecret").Once()

	op.handleAddSecret(&bindingSecret)

	annotationHelper.AssertCalled(t, "addAnnotationToMobileClient", "myClientId", "android", "http://example.org/#/app/myPushApplicationId/variants/myVariantId", "myServiceInstanceName")
	kubeHelper.AssertCalled(t, "updateSecret", mock.MatchedBy(func(secret *v1.Secret) bool {
		// Annotation for Android should be deleted
		if !reflect.DeepEqual(secret.Annotations, map[string]string{
			"binding/android": "myServiceBindingId",
			"binding/ios":     "toBeKept",
		}) {
			return false
		}

		if string(secret.Data["uri"]) != "http://example.org" ||
			string(secret.Data["name"]) != "ups" ||
			string(secret.Data["type"]) != "push" {
			return false;
		}

		if string(secret.Data["config"]) != "{\"android\":{\"senderId\":\"myProjectNumber\",\"variantId\":\"myVariantId\",\"variantSecret\":\"myVariantSecret\"},\"ios\":{\"variantId\":\"yourVariantId\",\"pop\":\"cake\"}}" {
			return false;
		}

		return true
	}))

	kubeHelper.AssertCalled(t, "deleteSecret", "myBindingSecret")

	kubeHelper.AssertExpectations(t)
	annotationHelper.AssertExpectations(t)
}


func TestConfigOperator_handleAddSecret_whenIOS(t *testing.T) {
	setup()

	bindingSecret := BindingSecret{
		Data: map[string][]byte{
			"appType":             []byte("IOS"),
			"clientId":            []byte("myClientId"),
			"googleKey":           []byte("myGoogleKey"),
			"projectNumber":       []byte("myProjectNumber"),
			"serviceBindingId":    []byte("myServiceBindingId"),
			"serviceInstanceName": []byte("myServiceInstanceName"),
		},
	}
	bindingSecret.Labels = map[string]string{
		"secretType": "mobile-client-binding-secret",
	}
	bindingSecret.Name = "myBindingSecret"

	pushClient.On("getServiceInstanceId").Return("myPushServiceInstanceId")
	pushClient.On("getApplicationId").Return("myPushApplicationId")
	pushClient.On("getBaseUrl").Return("http://example.org")
	pushClient.On("createIOSVariant", mock.Anything).Return(true, &IOSVariant{
		Certificate: []byte("myCertificate"),
		Passphrase:     "myPassphrase",
		Variant: Variant{
			Name:      "myIOSVariant",
			VariantID: "myVariantId",
			Secret:    "myVariantSecret",
		},
	})

	// no existing client config
	kubeHelper.On("findMobileClientConfig", "myClientId").Return(nil)

	configSecret := &v1.Secret{
		Data: map[string][]byte{
			"serviceInstanceName": []byte("myServiceInstanceName"),
			"config":              []byte("{\"android\":{\"variantId\":\"yourVariantId\",\"pop\":\"cake\"}}"),
		},
	}
	configSecret.Name = "mySecretName"
	configSecret.Annotations = map[string]string{
		"binding/android": "toBeKept",
	}

	kubeHelper.On("createClientConfigSecret", "myClientId", "myServiceInstanceName", "myPushServiceInstanceId", "myPushApplicationId").Return(configSecret)
	annotationHelper.On("addAnnotationToMobileClient", "myClientId", "ios", "http://example.org/#/app/myPushApplicationId/variants/myVariantId", "myServiceInstanceName").Once()
	kubeHelper.On("updateSecret", mock.Anything).Return(nil, nil)
	kubeHelper.On("deleteSecret", "myBindingSecret").Once()

	op.handleAddSecret(&bindingSecret)

	annotationHelper.AssertCalled(t, "addAnnotationToMobileClient", "myClientId", "ios", "http://example.org/#/app/myPushApplicationId/variants/myVariantId", "myServiceInstanceName")
	kubeHelper.AssertCalled(t, "updateSecret", mock.MatchedBy(func(secret *v1.Secret) bool {
		// Annotation for Android should be deleted
		if !reflect.DeepEqual(secret.Annotations, map[string]string{
			"binding/android": "toBeKept",
			"binding/ios":     "myServiceBindingId",
		}) {
			return false
		}

		if string(secret.Data["uri"]) != "http://example.org" ||
			string(secret.Data["name"]) != "ups" ||
			string(secret.Data["type"]) != "push" {
			return false;
		}

		if string(secret.Data["config"]) != "{\"android\":{\"variantId\":\"yourVariantId\",\"pop\":\"cake\"},\"ios\":{\"variantId\":\"myVariantId\",\"variantSecret\":\"myVariantSecret\"}}" {
			return false;
		}

		return true
	}))

	kubeHelper.AssertCalled(t, "deleteSecret", "myBindingSecret")

	kubeHelper.AssertExpectations(t)
	annotationHelper.AssertExpectations(t)
}
