package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete;create
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
		if !hasTorSidecarContainer(pod) {
			log.Info("Deleting unhidden pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil {
				return reconcile.Result{}, err
			}

			newPod := createPodWithSidecar(pod, torBridgeConfig.Spec.Image, torrc, torBridgeConfig.Spec.OrPort, torBridgeConfig.Spec.DirPort)
			log.Info("Creating new pod", "newPod.Name", newPod.Name, "newPod.Namespace", newPod.Namespace)
			if err = r.Create(ctx, newPod); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func createPodWithSidecar(pod corev1.Pod, image, torrc string, orPort, dirPort int) *corev1.Pod {
	newPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", pod.Name, "hidden"),
			Namespace: pod.Namespace,
		},
	}
	newPod.Spec = *pod.Spec.DeepCopy()

	sidecarContainer := makeSidecarContainer(image, torrc, orPort, dirPort)
	newPod.Spec.Containers = append(newPod.Spec.Containers, *sidecarContainer)

	return newPod
}

func hasTorSidecarContainer(pod corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == "tor-bridge" {
			return true
		}
	}
	return false
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
