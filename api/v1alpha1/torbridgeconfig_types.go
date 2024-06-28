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

// TorBridgeConfigSpec defines the desired state of TorBridgeConfig
type TorBridgeConfigSpec struct {
	RelayType                 RelayType `json:"relayType,omitempty"`
	OrPort                    int       `json:"orPort,omitempty"`
	DirPort                   int       `json:"dirPort,omitempty"`
	Image                     string    `json:"image,omitempty"`
	ContactInfo               string    `json:"contactInfo,omitempty"`
	Nickname                  string    `json:"nickname,omitempty"`
	ServerTransportPlugin     string    `json:"serverTransportPlugin,omitempty"`
	ServerTransportListenAddr string    `json:"serverTransportListenAddr,omitempty"`
	ExtOrPort                 string    `json:"extOrPort,omitempty"`
}

// TorBridgeConfigStatus defines the observed state of TorBridgeConfig
type TorBridgeConfigStatus struct {
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TorBridgeConfig is the Schema for the TorBridgeConfigs API
type TorBridgeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TorBridgeConfigSpec   `json:"spec,omitempty"`
	Status TorBridgeConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TorBridgeConfigList contains a list of TorBridgeConfig
type TorBridgeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TorBridgeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TorBridgeConfig{}, &TorBridgeConfigList{})
}
