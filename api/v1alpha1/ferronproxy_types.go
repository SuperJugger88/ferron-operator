/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FerronProxySpec defines the desired state of FerronProxy
type FerronProxySpec struct {
	ProxySelector string            `json:"proxySelector,omitempty"`
	Config        FerronProxyConfig `json:"config"`
	TLS           *FerronProxyTLS   `json:"tls,omitempty"`
}

type FerronProxyTLS struct {
	SecretName  string `json:"secretName,omitempty"`
	Certificate string `json:"certificate,omitempty"`
}

type FerronProxyConfig struct {
	Routes []FerronProxyConfigRoute `json:"routes"`
}

type FerronProxyConfigRoute struct {
	Host   string                   `json:"host"`
	Handle []FerronProxyRouteHandle `json:"handle"`
}

type FerronProxyRouteHandle struct {
	Location string                 `json:"location"`
	Proxy    FerronProxyHandleProxy `json:"proxy"`
}

type FerronProxyHandleProxy struct {
	Service FerronProxyProxyService `json:"service"`
}

type FerronProxyProxyService struct {
	Name     string                 `json:"name,omitempty"`
	Port     FerronProxyServicePort `json:"port,omitempty"`
	Scheme   string                 `json:"scheme,omitempty"`
	Endpoint string                 `json:"endpoint,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.name) && !has(self.number)) || (!has(self.name) && has(self.number))",message="exactly one of name or port must be specified"
type FerronProxyServicePort struct {
	Name   string `json:"name,omitempty"`
	Number int    `json:"number"`
}

// FerronProxyStatus defines the observed state of FerronProxy.
type FerronProxyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the FerronProxy resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FerronProxy is the Schema for the FerronProxyes API
type FerronProxy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of FerronProxy
	// +required
	Spec FerronProxySpec `json:"spec"`

	// status defines the observed state of FerronProxy
	// +optional
	Status FerronProxyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// FerronProxyList contains a list of FerronProxy
type FerronProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []FerronProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FerronProxy{}, &FerronProxyList{})
}
