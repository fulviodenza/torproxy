package controller

import (
	"context"
	"fmt"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	"github.com/fulviodenza/torproxy/internal/torrc"
	"github.com/fulviodenza/torproxy/internal/utils"
	"github.com/go-logr/logr"
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

// +kubebuilder:rbac:groups=tor.stack.io,resources=torbridgeconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tor.stack.io,resources=torbridgeconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tor.stack.io,resources=torbridgeconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups=apps,resources=replicaset,verbs=get;list;watch;update;patch;delete;create

func (r *TorBridgeConfigReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("got new request: ", "namespace/name", req.NamespacedName.String())

	torBridgeConfig := &v1beta1.TorBridgeConfig{}
	err := r.Get(ctx, req.NamespacedName, torBridgeConfig)
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	if errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}

	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(req.Namespace), client.MatchingLabels{"tor": "hide-me"}); err != nil {
		return reconcile.Result{}, err
	}

	if torBridgeConfig.DeletionTimestamp != nil {
		if err := r.unhandlePods(log, ctx, podList.Items); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if err := r.handlePods(log, ctx, podList.Items, *torBridgeConfig); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (r *TorBridgeConfigReconciler) unhandlePods(log logr.Logger, ctx context.Context, pods []corev1.Pod) error {
	for _, p := range pods {
		r.unhandlePod(log, ctx, p)
	}
	return nil
}

func (r *TorBridgeConfigReconciler) unhandlePod(log logr.Logger, ctx context.Context, pod corev1.Pod) error {
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

func (r *TorBridgeConfigReconciler) handlePods(log logr.Logger, ctx context.Context, pods []corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) error {
	for _, pod := range pods {
		if err := r.handlePod(log, ctx, pod, torBridgeConfig); err != nil {
			return err
		}
	}
	return nil
}

func (r *TorBridgeConfigReconciler) handlePod(log logr.Logger, ctx context.Context, pod corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) error {
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
			return r.handleControlledPod(ctx, pod, torBridgeConfig)
		}
	}
	return nil
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

func (r *TorBridgeConfigReconciler) handleControlledPod(ctx context.Context, pod corev1.Pod, torBridgeConfig v1beta1.TorBridgeConfig) error {
	for _, o := range pod.OwnerReferences {
		switch {
		case o.Kind == "Deployment":
			return r.handleDeployment(ctx, types.NamespacedName{
				Name:      o.Name,
				Namespace: pod.Namespace,
			}, torBridgeConfig)
		case o.Kind == "ReplicaSet":
			return r.handleReplicaSet(ctx, types.NamespacedName{
				Name:      o.Name,
				Namespace: pod.Namespace,
			}, torBridgeConfig)
		}
	}
	return nil
}

func (r *TorBridgeConfigReconciler) handleReplicaSet(ctx context.Context, ns types.NamespacedName, torBridgeConfig v1beta1.TorBridgeConfig) error {
	replicaSet := &appsv1.ReplicaSet{}
	err := r.Get(ctx, ns, replicaSet)
	if err != nil {
		return err
	}

	controllerName := ""
	for _, o := range replicaSet.OwnerReferences {
		if o.Kind == "Deployment" {
			controllerName = o.Name
		}
	}
	return r.handleDeployment(ctx, types.NamespacedName{Namespace: replicaSet.Namespace, Name: controllerName}, torBridgeConfig)
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
	err = r.Update(ctx, deployment)
	if err != nil {
		return err
	}
	return nil
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
	torrc := torrc.GenerateTorrc(torBridgeConfig.Spec)
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
		Owns(&corev1.Pod{}).
		Complete(r)
}
