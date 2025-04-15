package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MiddleGuardRelay is the API type to setup a tor guard/middle relay
type MiddleGuardRelay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MiddleGuardRelaySpec   `json:"spec,omitempty"`
	Status MiddleGuardRelayStatus `json:"status,omitempty"`
}

type MiddleGuardRelaySpec struct {
	// +required
	Nickname string `json:"nickname"`
	// +required
	ContactInfo string `json:"contactInfo"`
	// +required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ORPort int `json:"orPort"`
}

type MiddleGuardRelayStatus struct {
}

// +kubebuilder:object:root=true

type MiddleGuardRelayList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Items             []MiddleGuardRelay `json:"items"`
}

// Example Tor Circuit:
// Tor User -> Guard Relay -> Middle Relay -> Exit Relay -> Destination (i.e example.com)

// When using a bridge:

// Tor User -> Bridge Relay -> Middle Relay -> Exit Relay -> Destination (i.e example.com)

// Middle Guard relay

// Nickname    myNiceRelay  # Change "myNiceRelay" to something you like
// ContactInfo your@e-mail  # Write your e-mail and be aware it will be published
// ORPort      443          # You might use a different port, should you want to
// ExitRelay   0
// SocksPort   0

// Exit Relays

// We assume you read through the relay guide and technical considerations already. This subpage is for operators that want to turn on exiting on their relay.

// It is recommended that you setup exit relays on servers dedicated to this purpose. It is not recommended to install Tor exit relays on servers that you need for other services as well. Do not mix your own traffic with your exit relay traffic.

// Reverse DNS and WHOIS record
// Before turning your non-exit relay into an exit relay, ensure that you have set a reverse DNS record (PTR) to make it more obvious that this is a tor exit relay. Something like "tor-exit" in its name is a good start.

// If your provider offers it, make sure your WHOIS record contains clear indications that this is a Tor exit relay.

// Do use a domain name that you own. Definitely do not use torproject.org as a domain name for your reverse DNS.

// Exit Notice HTML page
// To make it even more obvious that this is a Tor exit relay you should serve a Tor exit notice HTML page. Tor can do that for you: if your DirPort is on TCP port 80, you can make use of tor's DirPortFrontPage feature to display an HTML file on that port. This file will be shown to anyone directing their browser to your Tor exit relay IP address.

// If you didn't set this up before, the following configuration lines must be applied to your torrc:

// DirPort 80
// DirPortFrontPage /path/to/html/file
// We offer a sample Tor exit notice HTML file, but you might want to adjust it to your needs.

// We also have a great blog post with some more tips for running an exit relay.

// Note: DirPort is deprecated since Tor 0.4.6.5, and self-tests are no longe being showed on tor's logs. For more information read its release notes and ticket #40282.

// Exit policy
// Defining the exit policy is one of the most important parts of an exit relay configuration. The exit policy defines which destination ports you are willing to forward. This has an impact on the amount of abuse emails you will get (less ports means less abuse emails, but an exit relay allowing only few ports is also less useful). If you want to be a useful exit relay you must at least allow destination ports 80 and 443.

// As a new exit relay - especially if you are new to your hoster - it is good to start with a reduced exit policy (to reduce the amount of abuse emails) and further open it up as you become more experienced. The reduced exit policy can be found on the Reduced Exit Policy wiki page.

// To become an exit relay change ExitRelay from 0 to 1 in your torrc configuration file and restart the tor daemon.

// ExitRelay 1
// DNS on Exit Relays
// Unlike other types of relays, exit relays also do DNS resolution for Tor clients. DNS resolution on exit relays is crucial for Tor clients and it should be reliable and fast by using caching.

// DNS resolution can have a significant impact on the performance and reliability that your exit relay provides.
// Don't use any of the big DNS resolvers (Google, OpenDNS, Quad9, Cloudflare, 4.2.2.1-6) as your primary or fallback DNS resolver to avoid centralization.
// We recommend running a local caching and DNSSEC-validating resolver without using any forwarders (specific instructions follow below, for various operating systems).
// If you want to add a second DNS resolver as a fallback to your /etc/resolv.conf configuration, choose a resolver within your autonomous system and make sure that it is not your first entry in that file (the first entry should be your local resolver).
// If a local resolver like unbound is not an option for you, use a resolver that your provider runs in the same autonomous system (to find out if an IP address is in the same AS as your relay, you can look it up using bgp.he.net).
// Avoid adding more than two resolvers to your /etc/resolv.conf file to limit AS-level exposure of DNS queries.
// Ensure your local resolver does not use any outbound source IP address that is used by any Tor exit or non-exits, because it is not uncommon that Tor IPs are (temporarily) blocked and a blocked DNS resolver source IP address can have a broad impact. For unbound you can use the outgoing-interface option to specify the source IP addresses for contacting other DNS servers.
// Large exit operators (>=100 Mbit/s) should make an effort to monitor and optimize Tor's DNS resolution timeout rate. This can be achieved via Tor's Prometheus exporter (MetricsPort). The following metric can be used to monitor the timeout rate as seen by Tor:
// tor_relay_exit_dns_error_total{reason="timeout"} 0
// There are multiple options for DNS server software. Unbound has become a popular one but feel free to use any other software that you are comfortable with. When choosing your DNS resolver software, make sure that it supports DNSSEC validation and QNAME minimization (RFC7816). Install the resolver software over your operating system's package manager, to ensure that it is updated automatically.

// By using your own DNS resolver, you are less vulnerable to DNS-based censorship that your upstream resolver might impose.

// Below are instructions on how to install and configure Unbound - a DNSSEC-validating and caching resolver - on your exit relay. Unbound has many configuration and tuning knobs, but we keep these instructions simple and short; the basic setup will do just fine for most operators.

// After switching to Unbound, verify that it works as expected by resolving a valid hostname. If it does not work, you can restore your old /etc/resolv.conf file.

// 1. Package installation
// The following commands install unbound, backup your DNS configuration, and tell the system to use the local resolver:

// # apt install unbound
// # cp /etc/resolv.conf /etc/resolv.conf.backup
// # echo nameserver 127.0.0.1 > /etc/resolv.conf
// 2. Lock changes in
// To avoid unwanted configuration changed (for example by the DHCP client):

// # chattr +i /etc/resolv.conf
// 3. QNAME minimisation
// The latest versions of unbound have qname-minimisation enabled by default. However, it is advisable to verify this setting, as older versions did not enable it automatically.

// To check and configure it, open the unbound configuration file, located at /etc/unbound/unbound.conf, look for the following entry and change it if necessary:

// server:
//     ...
//     qname-minimisation: yes
//     ...
// If the setting is missing, it should still be enabled by default. The Unbound resolver you just installed also does DNSSEC validation.

// 4. Start the service
// To enable and start the unbound service, run:

// # systemctl enable --now unbound
// If you are running systemd-resolved with its stub listener, you may need to do a bit more than just that. Please refer to the resolved.conf manpage.

// Types of relays on the Tor network

// All types of relays are important, but they have different technical requirements and potential legal implications. Understanding the different kinds of relays is the first step to learning which one is right for you.

// Example Tor Circuit:
// Tor User -> Guard Relay -> Middle Relay -> Exit Relay -> Destination (i.e example.com)

// When using a bridge:

// Tor User -> Bridge Relay -> Middle Relay -> Exit Relay -> Destination (i.e example.com)

// Guard and middle relays
// (also known as non-exit relays)

// A guard relay is the first relay (hop) in a Tor circuit. A middle relay is a relay that acts as the second hop in the Tor circuit. To become a guard relay, the relay has to be stable and fast (at least 2MByte/s of upstream and downstream bandwidth) otherwise it will remain a middle relay.

// Guard and middle relays usually do not receive abuse complaints. However, all relays are listed in the public Tor relay directory, and as a result, they may be blocked by certain services. These include services that either misunderstand how Tor works or deliberately want to censor Tor users, for example, online banking and streaming services.

// A non-exit Tor relay requires minimal maintenance efforts and bandwidth usage can be highly customized in the Tor configuration. The so called "exit policy" of the relay decides if it is a relay allowing clients to exit or not. A non-exit relay does not allow exiting in its exit policy.

// Important: If you are running a relay from home with a single static IP address and are concerned about your IP being blocked by certain online services, consider running a bridge or a Tor snowflake proxy instead. This alternative can help prevent your non-Tor traffic from being mistakenly blocked as though it's coming from a Tor relay.

// Exit relay
// The exit relay is the final relay in a Tor circuit, the one that sends traffic out to its destination. The services Tor clients are connecting to (website, chat service, email provider, etc) will see the IP address of the exit relay instead of the real IP address of the Tor user.

// Exit relays have the greatest legal exposure and liability of all the relays. For example, if a user downloads copyrighted material while using your exit relay, you, the operator may receive a DMCA notice. Any abuse complaints about the exit will go directly to you (via your hosting provider, depending on the WHOIS records). Generally, most complaints can be handled pretty easily through template letters.

// Because of the legal exposure that comes with running an exit relay, you should not run a Tor exit relay from your home. Ideal exit relay operators are affiliated with some institution, like a relay association, a university, a library, a hackerspace or a privacy related organization. An institution can not only provide greater bandwidth for the exit, but is better positioned to handle abuse complaints or the rare law enforcement inquiry.

// If you are considering running an exit relay, please read the section on legal considerations for exit relay operators.

// Bridge
// The design of the Tor network means that the IP addresses of Tor relays (guard, middle, and exit) are public. However, one of the ways Tor can be blocked by governments or ISPs is by blocklisting the IP addresses of these public Tor relays. Tor bridges are relays in the network that are not listed in the public Tor directory, which makes it harder for ISPs and governments to block them.

// Bridges are useful for Tor users under oppressive regimes or for people who want an extra layer of security because they're worried somebody will recognize that they are contacting a public Tor relay IP address. Several countries, including China and Iran, have found ways to detect and block connections to Tor bridges. Pluggable transports, a special kind of bridge, address this by adding an additional layer of obfuscation.

// Bridges are relatively easy, low-risk and low bandwidth Tor nodes to operate, but they have a big impact on users. A bridge isn't likely to receive any abuse complaints, and since bridges are not listed as public relays, they are unlikely to be blocked by popular services.

// Bridges are a great option if you can only run a Tor node from your home network, have only one static IP, and don't have a large amount of bandwidth to donate -- we recommend giving your bridge at least 1 Mbit/sec of bandwidth.

// Please see the relay requirements page to learn about the technical requirements for each relay type.
