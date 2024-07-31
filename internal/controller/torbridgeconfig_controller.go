package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	"github.com/fulviodenza/torproxy/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var TorContainerName = "tor-bridge"

type TorBridgeConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=torbridgeconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=torbridgeconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tor.fulvio.dev,resources=torbridgeconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch;delete;create
func (r *TorBridgeConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	torBridgeConfig := &v1beta1.TorBridgeConfig{}
	err := r.Get(ctx, req.NamespacedName, torBridgeConfig)
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if err == nil {
		podList := &corev1.PodList{}
		if err := r.List(ctx, podList, client.InNamespace(req.Namespace), client.MatchingLabels{"tor": "hide-me"}); err != nil {
			return reconcile.Result{}, err
		}

		if torBridgeConfig.DeletionTimestamp != nil {
			if err := r.unhandlePods(ctx, podList.Items); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		if err := r.handlePods(ctx, podList.Items, *torBridgeConfig); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// error is not found
	pod := &corev1.Pod{}
	err = r.Get(ctx, req.NamespacedName, pod)
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}
	if _, ok := pod.Labels["tor"]; !ok {
		return reconcile.Result{}, nil
	}
	if err == nil {
		torConfigName := pod.Labels["tor-config-name"]
		torConfigNamespace := pod.Labels["tor-config-namespace"]
		torBridgeConfig := &v1beta1.TorBridgeConfig{}
		if err := r.Get(ctx, types.NamespacedName{
			Namespace: torConfigNamespace,
			Name:      torConfigName,
		}, torBridgeConfig); err != nil && !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		if errors.IsNotFound(err) || torBridgeConfig.Name == "" {
			if err := r.unhandlePod(ctx, *pod); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		if err := r.handlePod(ctx, *pod, *torBridgeConfig); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

func (r *TorBridgeConfigReconciler) unhandlePods(ctx context.Context, pods []corev1.Pod) error {
	for _, p := range pods {
		r.unhandlePod(ctx, p)
	}
	return nil
}

func (r *TorBridgeConfigReconciler) unhandlePod(ctx context.Context, pod corev1.Pod) error {
	log := log.FromContext(ctx)
	switch {
	case hasTorContainer(pod):
		log.Info("Deleting unmanaged hidden pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		if err := r.Delete(ctx, &pod); err != nil {
			return err
		}
		newPod := createPod(pod)
		log.Info("Creating new unhidden pod", "newPod.Name", newPod.Name, "newPod.Namespace", newPod.Namespace)
		if err := r.Create(ctx, newPod); err != nil {
			return err
		}
	}
	return nil
}

func (r *TorBridgeConfigReconciler) handlePods(ctx context.Context, pods []corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) error {
	for _, pod := range pods {
		if err := r.handlePod(ctx, pod, torBridgeConfig); err != nil {
			return err
		}
	}
	return nil
}

func (r *TorBridgeConfigReconciler) handlePod(ctx context.Context, pod corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) error {
	log := log.FromContext(ctx)
	if !hasTorContainer(pod) && pod.DeletionTimestamp == nil {
		switch {
		case len(pod.OwnerReferences) == 0:
			log.Info("Deleting unhidden pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			if err := r.Delete(ctx, &pod); err != nil {
				return err
			}
			newPod := createPodWithTorContainer(pod, torBridgeConfig)
			log.Info("Creating new hidden pod", "newPod.Name", newPod.Name, "newPod.Namespace", newPod.Namespace)
			if err := r.Create(ctx, newPod); err != nil {
				return err
			}
		case len(pod.OwnerReferences) != 0:
			torBridgeConfig, err := r.getTorBridgeConfigFromPod(ctx, pod)
			if err != nil {
				return err
			}
			r.handleControlledPod(ctx, pod, *torBridgeConfig)
		}
	}
	return nil
}

func (r *TorBridgeConfigReconciler) getTorBridgeConfigFromPod(ctx context.Context, pod corev1.Pod) (*v1beta1.TorBridgeConfig, error) {
	torBridgeConfigName := pod.Labels["tor-config-name"]
	torBridgeConfigNamespace := pod.Labels["tor-config-namespace"]
	if torBridgeConfigName == "" || torBridgeConfigNamespace == "" {
		return nil, fmt.Errorf("pod %s/%s is missing tor-config-name or tor-config-namespace label", pod.Namespace, pod.Name)
	}

	torBridgeConfig := &v1beta1.TorBridgeConfig{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      torBridgeConfigName,
		Namespace: torBridgeConfigNamespace,
	}, torBridgeConfig)
	if err != nil {
		return nil, err
	}
	return torBridgeConfig, nil
}

func createPod(pod corev1.Pod) *corev1.Pod {
	newPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", pod.Name, "hidden", utils.GenerateName()),
			Namespace: pod.Namespace,
		},
	}
	return newPod
}

func createPodWithTorContainer(pod corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) *corev1.Pod {
	newPod := &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", pod.Name, "hidden", utils.GenerateName()),
			Namespace: pod.Namespace,
			Labels: map[string]string{
				"tor-config-name":      torBridgeConfig.Name,
				"tor-config-namespace": torBridgeConfig.Namespace,
			},
		},
	}
	newPod.Spec = *pod.Spec.DeepCopy()
	SOCKSPort := torBridgeConfig.Spec.SOCKSPort

	for i, container := range newPod.Spec.Containers {
		if container.Name != TorContainerName {
			newPod.Spec.Containers[i].Env = append(newPod.Spec.Containers[i].Env, corev1.EnvVar{
				Name:  "http_proxy",
				Value: fmt.Sprintf("socks5://127.0.0.1:%d", SOCKSPort),
			}, corev1.EnvVar{
				Name:  "https_proxy",
				Value: fmt.Sprintf("socks5://127.0.0.1:%d", SOCKSPort),
			}, corev1.EnvVar{
				Name:  "all_proxy",
				Value: fmt.Sprintf("socks5://127.0.0.1:%d", SOCKSPort),
			})
		}
	}

	torContainer := makeTorContainer(torBridgeConfig)
	newPod.Spec.Containers = append(newPod.Spec.Containers, *torContainer)

	return newPod
}

func (r *TorBridgeConfigReconciler) handleControlledPod(ctx context.Context, pod corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) {
	for _, o := range pod.OwnerReferences {
		switch {
		case o.Kind == "Deployment":
			r.handleDeployment(ctx, types.NamespacedName{
				Name:      o.Name,
				Namespace: pod.Namespace,
			}, torBridgeConfig)
		case o.Kind == "ReplicaSet":
			r.handleReplicaSet(ctx, types.NamespacedName{
				Name:      o.Name,
				Namespace: pod.Namespace,
			}, torBridgeConfig)
		}
	}
}

func (r *TorBridgeConfigReconciler) handleReplicaSet(ctx context.Context, ns types.NamespacedName, torBridgeConfig v1beta1.TorBridgeConfig) error {
	replicaSet := &appsv1.ReplicaSet{}
	err := r.Get(ctx, ns, replicaSet)
	if err != nil {
		return err
	}
	newReplicaSet := replicaSet.DeepCopy()

	torContainer := makeTorContainer(torBridgeConfig)
	newReplicaSet.Spec.Template.Spec.Containers = append(newReplicaSet.Spec.Template.Spec.Containers, *torContainer)
	newReplicaSet.Spec.Template.Name = fmt.Sprintf("%s-%s-%s", replicaSet.Name, "hidden", utils.GenerateName())
	return r.Update(ctx, replicaSet)
}

func (r *TorBridgeConfigReconciler) handleDeployment(ctx context.Context, ns types.NamespacedName, torBridgeConfig v1beta1.TorBridgeConfig) error {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, ns, deployment)
	if err != nil {
		return err
	}

	newDeployment := deployment.DeepCopy()

	torContainer := makeTorContainer(torBridgeConfig)
	newDeployment.Spec.Template.Spec.Containers = append(newDeployment.Spec.Template.Spec.Containers, *torContainer)
	newDeployment.Spec.Template.Name = fmt.Sprintf("%s-%s-%s", deployment.Name, "hidden", utils.GenerateName())
	return r.Update(ctx, deployment)
}

func hasTorContainer(pod corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == TorContainerName {
			return true
		}
	}
	return false
}

func makeTorContainer(torBridgeConfig v1beta1.TorBridgeConfig) *corev1.Container {
	torrc := generateTorrc(torBridgeConfig.Spec)
	return &corev1.Container{
		Name:  TorContainerName,
		Image: torBridgeConfig.Spec.Image,
		Command: []string{"sh", "-c", fmt.Sprintf(`
			echo '%s' > /etc/tor/torrc
			tor -f /etc/tor/torrc
		`, torrc)},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: int32(torBridgeConfig.Spec.OrPort),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				ContainerPort: int32(torBridgeConfig.Spec.DirPort),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				ContainerPort: int32(torBridgeConfig.Spec.SOCKSPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}
}

func (r *TorBridgeConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.TorBridgeConfig{}).
		Watches(&corev1.Pod{}, &handler.EnqueueRequestForObject{}).
		Watches(&appsv1.Deployment{}, &handler.EnqueueRequestForObject{}).
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
	if spec.SOCKSPort != 0 {
		sb.WriteString(fmt.Sprintf("SOCKSPort 0.0.0.0:%d\n", spec.SOCKSPort))
	}
	sb.WriteString("BridgeRelay 1\n")
	sb.WriteString("ExitPolicy reject *:*\n")

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
