package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CleanupTorBridgeConfigFinalizer = "torbridgeconfig.torproxy/cleanup"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Onion Address",type="string",JSONPath=".status.onionAddress"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

type OnionService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OnionServiceSpec   `json:"spec,omitempty"`
	Status OnionServiceStatus `json:"status,omitempty"`
}

type OnionServiceSpec struct {
	SOCKSPort int `json:"socksPort"`
	// Entry policies to allow/deny SOCKS requests based on IP address.
	// First entry that matches wins. If no SOCKSPolicy is set, we accept
	// all (and only) requests that reach a SOCKSPort. Untrusted users who
	// can access your SOCKSPort may be able to learn about the connections
	// you make.
	// SOCKSPolicy accept 192.168.0.0/16
	// SOCKSPolicy accept6 FC00::/7
	// SOCKSPolicy reject *
	SOCKSPolicy         []string `json:"socksPolicy,omitempty"`
	HiddenServicePort   int      `json:"hiddenServicePort"`
	HiddenServiceDir    string   `json:"hiddenServiceDir,omitempty"`
	HiddenServiceTarget string   `json:"hiddenServiceTarget,omitempty"`
}

type OnionServiceStatus struct {
	OnionAddress string `json:"onionAddress,omitempty"`
	Phase        string `json:"phase,omitempty"`
	Message      string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true

type OnionServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []OnionService `json:"items"`
}

// TorBridgeConfigSpec defines the desired state of TorBridgeConfig
type TorBridgeConfigSpec struct {
	OrPort                    int    `json:"orPort,omitempty"`
	DirPort                   int    `json:"dirPort,omitempty"`
	SOCKSPort                 int    `json:"socksPort,omitempty"`
	RedirectPort              int    `json:"redirectPort,omitempty"`
	Image                     string `json:"image,omitempty"`
	ContactInfo               string `json:"contactInfo,omitempty"`
	Nickname                  string `json:"nickname,omitempty"`
	ServerTransportPlugin     string `json:"serverTransportPlugin,omitempty"`
	ServerTransportListenAddr string `json:"serverTransportListenAddr,omitempty"`
	ExtOrPort                 string `json:"extOrPort,omitempty"`
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
	metav1.ListMeta `json:"metadata"`
	Items           []TorBridgeConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TorBridgeConfig{}, &TorBridgeConfigList{}, &OnionService{}, &OnionServiceList{})
}
