package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TorBridgeConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *TorBridgeConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	torBridgeConfig := &v1alpha1.TorBridgeConfig{}
	err := r.Get(ctx, req.NamespacedName, torBridgeConfig)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(req.Namespace), client.MatchingLabels{"tor": "hide-me"}); err != nil {
		return reconcile.Result{}, err
	}

	torrc := generateTorrc(torBridgeConfig.Spec)

	for _, pod := range podList.Items {
		if !hasTorContainer(pod) {
			log.Info("Adding Tor container", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)

			torContainer := makeSidecarContainer(torBridgeConfig.Spec.Image, torrc, torBridgeConfig.Spec.OrPort, torBridgeConfig.Spec.DirPort)

			sidecarContainer := makeIPTablesContainer(torBridgeConfig.Spec.RedirectPort, torBridgeConfig.Spec.OriginPort)

			pod.Spec.Containers = append(pod.Spec.Containers, *torContainer, *sidecarContainer)

			if err := r.Update(ctx, &pod); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

// TODO: instead of this we should evaluate
// the use of eBPF or https://pkg.go.dev/k8s.io/kubernetes/pkg/util/iptables
// the first is way more efficient but will be way more complex too
// the second is the implementation from kubernetes, that could actually
// be enough
func makeIPTablesContainer(redirectPort, originPort int) *corev1.Container {
	return &corev1.Container{
		Name:  "iptables-setup",
		Image: "alpine",
		Command: []string{"sh", "-c", fmt.Sprintf(`
			iptables -t nat -A PREROUTING -p tcp --dport %d -j REDIRECT --to-ports %d
		`, originPort, redirectPort)},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN"},
			},
		},
	}
}

func makeSidecarContainer(image, torrc string, orPort, dirPort int) *corev1.Container {
	return &corev1.Container{
		Name:  "tor-bridge",
		Image: image,
		Command: []string{"sh", "-c", fmt.Sprintf(`
			echo '%s' > /etc/tor/torrc
			tor -f /etc/tor/torrc
		`, torrc)},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: int32(orPort),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				ContainerPort: int32(dirPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}
}

func (r *TorBridgeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.TorBridgeConfig{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func hasTorContainer(pod corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == "tor-bridge" {
			return true
		}
	}
	return false
}

func generateTorrc(spec v1alpha1.TorBridgeConfigSpec) string {
	var sb strings.Builder

	sb.WriteString("Log notice stdout\n")
	sb.WriteString(fmt.Sprintf("ORPort %d\n", spec.OrPort))
	if spec.DirPort != 0 {
		sb.WriteString(fmt.Sprintf("DirPort %d\n", spec.DirPort))
	}
	if spec.RelayType == "bridge" {
		sb.WriteString("BridgeRelay 1\n")
		sb.WriteString("ExitPolicy reject *:*\n")
	}
	if spec.ServerTransportPlugin != "" {
		sb.WriteString(fmt.Sprintf("ServerTransportPlugin %s\n", spec.ServerTransportPlugin))
	}
	if spec.ServerTransportListenAddr != "" {
		sb.WriteString(fmt.Sprintf("ServerTransportListenAddr %s\n", spec.ServerTransportListenAddr))
	}
	if spec.ExtOrPort != "" {
		sb.WriteString(fmt.Sprintf("ExtORPort %s\n", spec.ExtOrPort))
	}
	if spec.ContactInfo != "" {
		sb.WriteString(fmt.Sprintf("ContactInfo %s\n", spec.ContactInfo))
	}
	if spec.Nickname != "" {
		sb.WriteString(fmt.Sprintf("Nickname %s\n", spec.Nickname))
	}

	return sb.String()
}
