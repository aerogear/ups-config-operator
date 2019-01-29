// Taken from https://github.com/aerogear/mobile-developer-console/blob/master/pkg/apis/aerogear/v1alpha1/types.go
package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MobileClientList is a list of MobileClient objects.
type MobileClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []MobileClient `json:"items" protobuf:"bytes,2,rep,name=items"`
}

type MobileClientSpec struct {
	ApiKey string `json:"apiKey"`
	DmzUrl string `json:"dmzUrl"`
	Name   string `json:"name,required"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MobileClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              MobileClientSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            MobileClientStatus `json:"status,omitempty"`
}

//for mobile-services.json
type MobileClientStatus struct {
	Services []MobileClientService `json:"services"`
}

type MobileClientService struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Url  string `json:"url"`
	//ideally we would like to use map[string]interface{} type here, but we can't as the generated code will complain that the interface{} is not `DeepCopy`-able.
	Config  json.RawMessage `json:"config"`
	Version string          `json:"version"`
}
