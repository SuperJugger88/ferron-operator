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

type FerronProxySelectorSpec struct {
	ControllerName string `json:"controllerName"`
	Image          string `json:"image,omitempty"`
	Replicas       *int32 `json:"replicas,omitempty"`
	ServiceType    string `json:"serviceType,omitempty"`
}

type FerronProxySelectorStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// FerronProxySelector is a cluster-scoped resource that selects FerronProxy resources by ingressClassName.
// Similar to IngressClass for nginx-ingress.
type FerronProxySelector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   FerronProxySelectorSpec   `json:"spec"`
	Status FerronProxySelectorStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// FerronProxySelectorList contains a list of FerronProxySelector
type FerronProxySelectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []FerronProxySelector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FerronProxySelector{}, &FerronProxySelectorList{})
}
