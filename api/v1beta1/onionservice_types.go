package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const CleanupOnionServiceFinalizer = "onionservice.torproxy/cleanup"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Onion Address",type="string",JSONPath=".status.onionAddress"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OnionService is the API type to create an onion service listening on the port
// SOCKSPort with the given SOCKSPolicies.
type OnionService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   OnionServiceSpec   `json:"spec,omitempty"`
	Status OnionServiceStatus `json:"status,omitempty"`
}

type OnionServiceSpec struct {
	// SOCKSPort specifies the port where Tor will listen for SOCKS connections from your applications.
	// If set to 0, no SOCKS listener will be started.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=65535
	SOCKSPort int `json:"socksPort"`

	// SOCKSPolicy controls which external applications can connect to which
	// ports on the SOCKS interface. The format is "[accept|reject] address/mask[:port]".
	// For example:
	// - "accept 192.168.0.0/16" - allow connections from addresses in 192.168.0.0/16
	// - "accept6 FC00::/7" - allow connections from IPv6 addresses in FC00::/7
	// - "reject *" - reject everything else
	// If no SOCKSPolicy entries are provided, Tor will accept all connections
	// on the SOCKS port by default.
	SOCKSPolicy []string `json:"socksPolicy,omitempty"`

	// HiddenServicePort specifies the virtual port that will be exposed on the .onion address.
	// Clients connecting to this port on the .onion address will be redirected to
	// the target specified in HiddenServiceTarget.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	HiddenServicePort int `json:"hiddenServicePort"`

	// HiddenServiceDir specifies the directory where Tor will store information about the hidden
	// service, including the private key. If not specified, defaults to "/var/lib/tor/hidden_service/".
	// This directory must be accessible only by the user running Tor and have permissions 0700.
	// +optional
	HiddenServiceDir string `json:"hiddenServiceDir,omitempty"`

	// HiddenServiceTarget specifies the IP address and port where the hidden service
	// traffic should be forwarded (e.g., "127.0.0.1:8080"). This is the backend service
	// that will be accessible through the .onion address.
	// Format should be "<address>:<port>".
	HiddenServiceTarget string `json:"hiddenServiceTarget,omitempty"`
}

type OnionServiceStatus struct {
	// OnionAddress is the generated .onion address for this hidden service, once available.
	// This is the address that clients can use to connect to the service over the Tor network.
	// Example: "abcdefghijklmnop.onion"
	OnionAddress string `json:"onionAddress,omitempty"`

	// Phase represents the current state of the OnionService (Pending, Running, Failed, etc).
	Phase string `json:"phase,omitempty"`

	// Message provides additional information about the current phase.
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true

type OnionServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []OnionService `json:"items"`
}
