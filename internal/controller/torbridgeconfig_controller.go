package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TorBridgeConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=torbridgeconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=torbridgeconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=torbridgeconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch
func (r *TorBridgeConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)

	torBridgeConfig := &v1beta1.TorBridgeConfig{}
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
		if !hasTorInitContainer(pod) {
			log.Info("Deleting unhidden pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil {
				return reconcile.Result{}, err
			}
			newPod := pod.DeepCopy()
			newPod.Name = fmt.Sprintf("%s-%s", pod.Name, "hidden")
			newPod.ResourceVersion = ""
			log.Info("Creating new pod:", "newPod.Name", newPod.Name, "newPod.Namespace", newPod.Namespace)
			initContainer := makeSidecarContainer(torBridgeConfig.Spec.Image, torrc, torBridgeConfig.Spec.OrPort, torBridgeConfig.Spec.DirPort)
			newPod.Spec.InitContainers = append(pod.Spec.InitContainers, *initContainer)
			if err = r.Create(ctx, newPod); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
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

func hasTorInitContainer(pod corev1.Pod) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Name == "tor-init" {
			return true
		}
	}
	return false
}

func (r *TorBridgeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.TorBridgeConfig{}).
		Watches(&corev1.Pod{}, &handler.EnqueueRequestForObject{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

func generateTorrc(spec v1beta1.TorBridgeConfigSpec) string {
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
