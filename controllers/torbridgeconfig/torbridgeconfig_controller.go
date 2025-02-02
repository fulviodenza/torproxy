package controllers

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	"github.com/fulviodenza/torproxy/controllers/utils"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
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

	if err == nil {
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

	// if error is not found we need to look for the pods
	// having the label "tor" and eventually inject the configuration
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
			if err := r.unhandlePod(log, ctx, *pod); err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil
		}

		if err := r.handlePod(log, ctx, *pod, *torBridgeConfig); err != nil {
			return reconcile.Result{}, err
		}
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
			// Log the Onion address for the new Pod
			return r.getOnionAddress(log, ctx, *newPod)
		case len(pod.OwnerReferences) != 0:
			torBridgeConfig, err := r.getTorBridgeConfigFromPod(ctx, pod)
			if err != nil {
				return err
			}
			if err := r.handleControlledPod(ctx, pod, *torBridgeConfig); err != nil {
				return err
			}
			// Log the Onion address for the controlled Pod
			return r.getOnionAddress(log, ctx, pod)
		}
	}
	return nil
}

// Function to retrieve the Onion service address
func (r *TorBridgeConfigReconciler) getOnionAddress(log logr.Logger, ctx context.Context, pod corev1.Pod) error {
	// Set up the Kubernetes client configuration
	kubeconfig := config.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		log.Error(err, "Failed to create Kubernetes client")
		return err
	}

	// Prepare the exec command to read the Onion address from /var/lib/tor/hidden_service/hostname
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		Param("container", "tor-bridge"). // Ensure this matches the Tor container name in your Pod spec
		Param("command", "cat").
		Param("command", "/var/lib/tor/hidden_service/hostname").
		Param("stderr", "true").
		Param("stdout", "true")

	// Set up the executor
	exec, err := remotecommand.NewSPDYExecutor(kubeconfig, "POST", req.URL())
	if err != nil {
		log.Error(err, "Failed to initialize SPDY executor")
		return err
	}

	// Capture the output in a buffer
	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		log.Error(err, "Failed to exec command in pod", "stderr", stderr.String())
		return err
	}

	// Log the Onion address
	onionAddress := stdout.String()
	log.Info("Onion service address", "address", onionAddress)
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

	// newReplicaSet := replicaSet.DeepCopy()

	// torContainer := makeTorContainer(torBridgeConfig)
	// newReplicaSet.Spec.Template.Spec.Containers = append(newReplicaSet.Spec.Template.Spec.Containers, *torContainer)
	// newReplicaSet.Spec.Template.Name = fmt.Sprintf("%s-%s-%s", replicaSet.Name, "hidden", utils.GenerateName())
	// return r.Update(ctx, replicaSet)
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
	torrc := generateTorrc(torBridgeConfig)
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

// Service for bridge configuration
//
// apiVersion: v1
// kind: Service
// metadata:
//   name: tor-bridge-service
//   namespace: <namespace>
// spec:
//   selector:
//     app: tor-bridge
//   ports:
//     - protocol: TCP
//       port: <external-port>       # External port to connect to
//       targetPort: <orport>        # Internal port the Tor bridge is listening on
//   type: ClusterIP  # Or LoadBalancer/NodePort if you need external access

// Service for Hidden Service configuration
//
// apiVersion: v1
// kind: Service
// metadata:
//   name: hidden-service-backend
//   namespace: <namespace>
// spec:
//   selector:
//     app: hidden-service
//   ports:
//     - protocol: TCP
//       port: 80
//       targetPort: <app-port>   # The internal port for the backend application
//   type: ClusterIP

// Service for Relay Configuration
//
// apiVersion: v1
// kind: Service
// metadata:
//   name: tor-relay-service
//   namespace: <namespace>
// spec:
//   selector:
//     app: tor-relay
//   ports:
//     - protocol: TCP
//       port: <external-orport>        # External port for ORPort
//       targetPort: <orport>           # Internal port for ORPort
//     - protocol: TCP
//       port: <external-dirport>       # External port for DirPort (if used)
//       targetPort: <dirport>          # Internal port for DirPort
//   type: LoadBalancer  # Can also be NodePort if LoadBalancer is unavailable

func generateTorrc(torConfig v1beta1.TorBridgeConfig) string {
	var sb strings.Builder

	sb.WriteString("Log notice stdout\n")

	switch torConfig.Spec.Mode {
	case "bridge":
		// Bridge configuration
		sb.WriteString("BridgeRelay 1\n")
		sb.WriteString(fmt.Sprintf("ORPort %d\n", torConfig.Spec.OrPort))
		sb.WriteString(fmt.Sprintf("SOCKSPort 127.0.0.1:%d\n", torConfig.Spec.SOCKSPort)) // Local access only

		if torConfig.Spec.ServerTransportPlugin != "" {
			sb.WriteString(fmt.Sprintf("ServerTransportPlugin %s\n", torConfig.Spec.ServerTransportPlugin))
		}
		if torConfig.Spec.ServerTransportListenAddr != "" {
			sb.WriteString(fmt.Sprintf("ServerTransportListenAddr %s\n", torConfig.Spec.ServerTransportListenAddr))
		}

		if torConfig.Spec.ExtOrPort != "" {
			sb.WriteString(fmt.Sprintf("ExtORPort %s\n", torConfig.Spec.ExtOrPort))
		}

		if torConfig.Spec.ContactInfo != "" {
			sb.WriteString(fmt.Sprintf("ContactInfo %s\n", torConfig.Spec.ContactInfo))
		}

		if torConfig.Spec.Nickname != "" {
			sb.WriteString(fmt.Sprintf("Nickname %s\n", torConfig.Spec.Nickname))
		}

	case "hidden_service":
		// Hidden Service configuration
		sb.WriteString("HiddenServiceDir /var/lib/tor/hidden_service/\n")
		sb.WriteString(fmt.Sprintf("HiddenServicePort %d 127.0.0.1:%d\n", 80, torConfig.Spec.OrPort))

		sb.WriteString(fmt.Sprintf("SOCKSPort 127.0.0.1:%d\n", torConfig.Spec.SOCKSPort)) // Only localhost access

		if torConfig.Spec.ContactInfo != "" {
			sb.WriteString(fmt.Sprintf("ContactInfo %s\n", torConfig.Spec.ContactInfo))
		}

		if torConfig.Spec.Nickname != "" {
			sb.WriteString(fmt.Sprintf("Nickname %s\n", torConfig.Spec.Nickname))
		}

	case "relay":
		// Relay configuration
		sb.WriteString(fmt.Sprintf("ORPort %d\n", torConfig.Spec.OrPort))

		if torConfig.Spec.DirPort != 0 {
			sb.WriteString(fmt.Sprintf("DirPort %d\n", torConfig.Spec.DirPort))
		}

		sb.WriteString(fmt.Sprintf("SOCKSPort 127.0.0.1:%d\n", torConfig.Spec.SOCKSPort)) // Only localhost access

		if torConfig.Spec.ExtOrPort != "" {
			sb.WriteString(fmt.Sprintf("ExtORPort %s\n", torConfig.Spec.ExtOrPort))
		}

		if torConfig.Spec.ContactInfo != "" {
			sb.WriteString(fmt.Sprintf("ContactInfo %s\n", torConfig.Spec.ContactInfo))
		}

		if torConfig.Spec.Nickname != "" {
			sb.WriteString(fmt.Sprintf("Nickname %s\n", torConfig.Spec.Nickname))
		}

		if torConfig.Spec.ExitRelay {
			sb.WriteString("ExitRelay 1\n")
		} else {
			sb.WriteString("ExitRelay 0\n")
		}
	default:
		// Default behavior or error
		sb.WriteString("Invalid mode specified\n")
	}

	return sb.String()
}
