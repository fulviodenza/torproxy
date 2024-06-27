/*
Copyright 2024.

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

type RelayType string

// TorConfigSpec defines the desired state of TorConfig
type TorConfigSpec struct {
	Image       string    `json:"image,omitempty"`
	RelayType   RelayType `json:"relayType,omitempty"`
	OrPort      int       `json:"orPort,omitempty"`
	DirPort     int       `json:"dirPort,omitempty"`
	ContactInfo string    `json:"contactInfo,omitempty"`
	Nickname    string    `json:"nickname,omitempty"`
}

// TorConfigStatus defines the observed state of TorConfig
type TorConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TorConfig is the Schema for the TorConfigs API
type TorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TorConfigSpec   `json:"spec,omitempty"`
	Status TorConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TorConfigList contains a list of TorConfig
type TorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TorConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TorConfig{}, &TorConfigList{})
}
