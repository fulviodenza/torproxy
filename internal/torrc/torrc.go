package torrc

import (
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1beta1"
)

// GenerateTorrcConfig generates torrc configuration based on OnionService spec
func GenerateTorrcConfigForMiddleGuardRelay(relay *v1beta1.MiddleGuardRelay) string {
	var config strings.Builder

	fmt.Fprintf(&config, "Nickname %s\n", relay.Spec.ContactInfo)
	fmt.Fprintf(&config, "ContactInfo %s\n", relay.Spec.Nickname)
	fmt.Fprintf(&config, "ORPort %d\n", relay.Spec.ORPort)

	fmt.Fprintf(&config, "ExitRelay 0\n")
	fmt.Fprintf(&config, "SocksPort 0\n")
	return config.String()
}

// GenerateTorrcConfig generates torrc configuration based on OnionService spec
func GenerateTorrcConfigForOnionService(onion *v1beta1.OnionService) string {
	var config strings.Builder

	if onion.Spec.SOCKSPort > 0 {
		fmt.Fprintf(&config, "SOCKSPort %d\n", onion.Spec.SOCKSPort)
	}

	for _, policy := range onion.Spec.SOCKSPolicy {
		fmt.Fprintf(&config, "SOCKSPolicy %s\n", policy)
	}

	hiddenServiceDir := onion.Spec.HiddenServiceDir
	if hiddenServiceDir == "" {
		hiddenServiceDir = "/var/lib/tor/hidden_service/"
	}

	fmt.Fprintf(&config, "HiddenServiceDir %s\n", hiddenServiceDir)
	fmt.Fprintf(&config, "HiddenServicePort %d %s\n",
		onion.Spec.HiddenServicePort,
		onion.Spec.HiddenServiceTarget)

	fmt.Fprintf(&config, "DataDirectory /var/lib/tor\n")
	fmt.Fprintf(&config, "RunAsDaemon 0\n")

	return config.String()
}
