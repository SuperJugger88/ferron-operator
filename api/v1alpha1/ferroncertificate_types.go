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

type FerronCertificateSpec struct {
	Domain     string `json:"domain"`
	Email      string `json:"email"`
	ACMEServer string `json:"acmeServer,omitempty"`
	SecretName string `json:"secretName,omitempty"`
}

type FerronCertificateStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// FerronCertificate is the Schema for the ferroncertificates API
type FerronCertificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   FerronCertificateSpec   `json:"spec"`
	Status FerronCertificateStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// FerronCertificateList contains a list of FerronCertificate
type FerronCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []FerronCertificate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FerronCertificate{}, &FerronCertificateList{})
}
