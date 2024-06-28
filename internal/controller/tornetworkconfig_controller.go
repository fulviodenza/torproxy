package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	torv1alpha1 "github.com/fulviodenza/torproxy/api/v1alpha1"
)

// TorNetworkConfigReconciler reconciles a TorNetworkConfig object
type TorNetworkConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=tornetworkconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=tornetworkconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=tornetworkconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *TorNetworkConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("tornetworkconfig", req.NamespacedName)

	torNetworkConfig := &torv1alpha1.TorNetworkConfig{}
	err := r.Get(ctx, req.NamespacedName, torNetworkConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Reconciling TorNetworkConfig", "config", torNetworkConfig)

	configMapName := "tor-config-" + torNetworkConfig.Name
	TorBridgeConfig := generateTorBridgeConfig(torNetworkConfig.Spec)

	configMap := &corev1.ConfigMap{
		ObjectMeta: ctrl.ObjectMeta{
			Namespace: req.Namespace,
			Name:      configMapName,
		},
		Data: map[string]string{
			"torrc": TorBridgeConfig,
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: req.Namespace}, existingConfigMap)
	if err != nil && errors.IsNotFound(err) {
		if err := r.Create(ctx, configMap); err != nil {
			log.Error(err, "Failed to create ConfigMap")
			return ctrl.Result{}, err
		}
	} else if err == nil {
		existingConfigMap.Data = configMap.Data
		if err := r.Update(ctx, existingConfigMap); err != nil {
			log.Error(err, "Failed to update ConfigMap")
			return ctrl.Result{}, err
		}
	} else {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(req.Namespace),
		client.MatchingLabels{"tor.fulvio.dev/inject": "true"},
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		log.Error(err, "Failed to list pods")
		return ctrl.Result{}, err
	}

	for _, pod := range podList.Items {
		hasTorSidecar := false
		for _, container := range pod.Spec.Containers {
			if container.Name == "tor-sidecar" {
				hasTorSidecar = true
				break
			}
		}

		if !hasTorSidecar {
			sidecar := corev1.Container{
				Name: "tor-sidecar",
				// TBD: define the image to pull from the
				// registry. This is the container that runs tor
				Image: "fulviodenza/torproxy:latest",
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 9050, // Tor SOCKS port
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "tor-config",
						MountPath: "/etc/tor",
					},
				},
			}

			pod.Spec.Containers = append(pod.Spec.Containers, sidecar)

			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
				Name: "tor-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			})

			if err := r.Update(ctx, &pod); err != nil {
				log.Error(err, "Failed to update pod with Tor sidecar")
				return ctrl.Result{}, err
			}
		}
	}

	torNetworkConfig.Status = torv1alpha1.TorNetworkConfigStatus{}
	if err := r.Status().Update(ctx, torNetworkConfig); err != nil {
		log.Error(err, "Failed to update TorNetworkConfig status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TorNetworkConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torv1alpha1.TorNetworkConfig{}).
		Complete(r)
}

func generateTorBridgeConfig(spec torv1alpha1.TorNetworkConfigSpec) string {
	var TorBridgeConfig strings.Builder

	TorBridgeConfig.WriteString("SOCKSPort 0.0.0.0:9050\n")
	TorBridgeConfig.WriteString(fmt.Sprintf("ExitNodes %s\n", formatExitNodes(spec.DefaultExitNodes)))

	for _, hs := range spec.HiddenServices {
		TorBridgeConfig.WriteString(fmt.Sprintf("HiddenServiceDir /var/lib/tor/hidden_service/%s\n", hs.Hostname))
		TorBridgeConfig.WriteString(fmt.Sprintf("HiddenServicePort %d\n", hs.TargetPort))
	}

	return TorBridgeConfig.String()
}

func formatExitNodes(exitNodes []torv1alpha1.ExitNode) string {
	var nodes []string
	for _, node := range exitNodes {
		nodes = append(nodes, node.Country)
	}
	return fmt.Sprintf("{%s}", strings.Join(nodes, ", "))
}
