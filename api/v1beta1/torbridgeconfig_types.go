package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CleanupTorBridgeConfigFinalizer = "torbridgeconfig.torproxy/cleanup"

// TorBridgeConfigSpec defines the desired state of TorBridgeConfig
type TorBridgeConfigSpec struct {
	OrPort                    int    `json:"orPort,omitempty"`
	DirPort                   int    `json:"dirPort,omitempty"`
	SOCKSPort                 int    `json:"socksPort,omitempty"`
	Image                     string `json:"image,omitempty"`
	ContactInfo               string `json:"contactInfo,omitempty"`
	Nickname                  string `json:"nickname,omitempty"`
	ServerTransportPlugin     string `json:"serverTransportPlugin,omitempty"`
	ServerTransportListenAddr string `json:"serverTransportListenAddr,omitempty"`
	ExtOrPort                 string `json:"extOrPort,omitempty"`
	OriginPort                int    `json:"originPort,omitempty"`
	RedirectPort              int    `json:"redirectPort,omitempty"`
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
