package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TorNetworkConfigSpec defines the desired state of TorNetworkConfig
type TorNetworkConfigSpec struct {
	DefaultExitNodes []ExitNode      `json:"defaultExitNodes,omitempty"`
	HiddenServices   []HiddenService `json:"hiddenServices,omitempty"`
	MetricsEnabled   bool            `json:"metricsEnabled,omitempty"`
}

// ExitNode represents a Tor exit node configuration
type ExitNode struct {
	Country string `json:"country,omitempty"`
}

// HiddenService represents a Tor hidden service configuration
type HiddenService struct {
	Hostname   string `json:"hostname,omitempty"`
	TargetPort int    `json:"targetPort,omitempty"`
}

// TorNetworkConfigStatus defines the observed state of TorNetworkConfig
type TorNetworkConfigStatus struct {
	// TBD
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TorNetworkConfig is the Schema for the tornetworkconfigs API
type TorNetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TorNetworkConfigSpec   `json:"spec,omitempty"`
	Status TorNetworkConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TorNetworkConfigList contains a list of TorNetworkConfig
type TorNetworkConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TorNetworkConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TorNetworkConfig{}, &TorNetworkConfigList{})
}
