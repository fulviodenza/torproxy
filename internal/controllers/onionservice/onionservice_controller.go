package onionservice

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	"github.com/fulviodenza/torproxy/internal/torrc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OnionServiceReconciler struct {
	client.Client
	KubeClient kubernetes.Interface
	Scheme     *runtime.Scheme
}

var TorDockerImage = "dperson/torproxy:latest"

const torFinalizerName = "onionservice.tor.stack.io/finalizer"

// +kubebuilder:rbac:groups=tor.stack.io,resources=onionservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tor.stack.io,resources=onionservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tor.stack.io,resources=onionservices/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups=apps,resources=replicaset,verbs=get;list;watch;update;patch;delete;create

func (r *OnionServiceReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("got new request: ", "namespace/name", req.NamespacedName.String())

	onionService := &v1beta1.OnionService{}
	err := r.Get(ctx, req.NamespacedName, onionService)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !onionService.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(onionService, torFinalizerName) {
			// if err := r.cleanupOnionService(ctx, onionService); err != nil {
			// 	return reconcile.Result{}, err
			// }

			controllerutil.RemoveFinalizer(onionService, torFinalizerName)
			if err := r.Update(ctx, onionService); err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(onionService, torFinalizerName) {
		controllerutil.AddFinalizer(onionService, torFinalizerName)
		if err := r.Update(ctx, onionService); err != nil {
			return reconcile.Result{}, err
		}
	}

	torrcConfig := torrc.GenerateTorrcConfigForOnionService(onionService)

	if err := r.reconcileConfigMap(ctx, onionService, torrcConfig); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcilePVC(ctx, onionService); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileDeployment(ctx, onionService); err != nil {
		return reconcile.Result{}, err
	}

	err = r.reconcileStatus(ctx, onionService)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// Create or update ConfigMap with torrc
func (r *OnionServiceReconciler) reconcileConfigMap(ctx context.Context, onion *v1beta1.OnionService, torrcConfig string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      onion.Name + "-torrc",
			Namespace: onion.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(onion, v1beta1.GroupVersion.WithKind("OnionService")),
			},
		},
		Data: map[string]string{
			"torrc": torrcConfig,
		},
	}

	found := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, cm)
	} else if err != nil {
		return err
	}

	if !reflect.DeepEqual(found.Data, cm.Data) {
		found.Data = cm.Data
		return r.Update(ctx, found)
	}

	return nil
}

// Create or update PVC for hidden service persistence
func (r *OnionServiceReconciler) reconcilePVC(ctx context.Context, onion *v1beta1.OnionService) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      onion.Name + "-hidden-service",
			Namespace: onion.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(onion, v1beta1.GroupVersion.WithKind("OnionService")),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Mi"),
				},
			},
		},
	}

	found := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, pvc)
	} else if err != nil {
		return err
	}

	return nil
}

func (r *OnionServiceReconciler) reconcileDeployment(ctx context.Context, onion *v1beta1.OnionService) error {
	hiddenServiceDir := onion.Spec.HiddenServiceDir
	if hiddenServiceDir == "" {
		hiddenServiceDir = "/var/lib/tor/hidden_service/"
	}

	torUID := int64(101)
	torGID := int64(101)
	var zero int64 = 0
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      onion.Name,
			Namespace: onion.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(onion, v1beta1.GroupVersion.WithKind("OnionService")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": onion.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": onion.Name,
					},
				},
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:  "init-permissions", // init container to set up permissions
							Image: "busybox",
							Command: []string{
								"sh",
								"-c",
								fmt.Sprintf("mkdir -p %s && chown -R 101:101 %s && chmod -R 700 %s",
									hiddenServiceDir, filepath.Dir(hiddenServiceDir), hiddenServiceDir),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "hidden-service",
									MountPath: filepath.Dir(hiddenServiceDir),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &zero,
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: &torGID,
					},
					Containers: []corev1.Container{
						{
							Name:  "tor",
							Image: TorDockerImage,
							Command: []string{
								"sh",
								"-c",
								"tor -f /etc/tor/torrc",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "socks",
									ContainerPort: int32(onion.Spec.SOCKSPort),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "torrc",
									MountPath: "/etc/tor/torrc",
									SubPath:   "torrc",
								},
								{
									Name:      "hidden-service",
									MountPath: filepath.Dir(hiddenServiceDir),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  &torUID,
								RunAsGroup: &torGID,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "torrc",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: onion.Name + "-torrc",
									},
								},
							},
						},
						{
							Name: "hidden-service",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: onion.Name + "-hidden-service",
								},
							},
						},
					},
				},
			},
		},
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		return r.Create(ctx, deployment)
	} else if err != nil {
		return err
	}

	found.Spec = deployment.Spec
	return r.Update(ctx, found)
}

func (r *OnionServiceReconciler) reconcileStatus(ctx context.Context, onion *v1beta1.OnionService) error {
	log := log.FromContext(ctx)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: onion.Name, Namespace: onion.Namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.updateStatus(ctx, onion, "Pending", "", "Deployment not yet created")
		}
		return err
	}

	if deployment.Status.ReadyReplicas == 0 {
		return r.updateStatus(ctx, onion, "Initializing", "", "Waiting for pod to become ready")
	}

	// If we already have an onion address, no need to fetch again
	if onion.Status.OnionAddress != "" && onion.Status.Phase == "Ready" {
		return nil
	}

	podList := &corev1.PodList{}
	err = r.List(ctx, podList, client.InNamespace(onion.Namespace), client.MatchingLabels{"app": onion.Name})
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return r.updateStatus(ctx, onion, "Initializing", "", "No pods found")
	}

	// Find a running pod
	var runningPod *corev1.Pod
	for i := range podList.Items {
		if podList.Items[i].Status.Phase == corev1.PodRunning {
			runningPod = &podList.Items[i]
			break
		}
	}

	if runningPod == nil {
		return r.updateStatus(ctx, onion, "Initializing", "", "Waiting for pod to start")
	}

	hiddenServiceDir := onion.Spec.HiddenServiceDir
	if hiddenServiceDir == "" {
		hiddenServiceDir = "/var/lib/tor/hidden_service"
	}
	hostnameFile := filepath.Join(hiddenServiceDir, "hostname")

	onionAddress, err := r.execInPod(ctx, runningPod.Name, runningPod.Namespace, "tor", []string{"cat", hostnameFile})
	if err != nil {
		log.Info("Failed to read onion address, will retry", "error", err.Error())
		return r.updateStatus(ctx, onion, "Initializing", "", "Waiting for Tor to generate .onion address")
	}

	onionAddress = strings.TrimSpace(onionAddress)
	if onionAddress == "" {
		return r.updateStatus(ctx, onion, "Initializing", "", "Onion address file is empty, waiting for Tor")
	}

	log.Info("Successfully retrieved onion address", "address", onionAddress)
	return r.updateStatus(ctx, onion, "Ready", onionAddress, "OnionService is ready")
}

// execInPod executes a command in a pod and returns the output
func (r *OnionServiceReconciler) execInPod(ctx context.Context, podName, namespace, containerName string, command []string) (string, error) {
	if r.KubeClient == nil {
		return "", fmt.Errorf("KubeClient is not initialized")
	}

	req := r.KubeClient.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, runtime.NewParameterCodec(r.Scheme))

	config, err := ctrl.GetConfig()
	if err != nil {
		return "", err
	}

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", err
	}

	var stdout, stderr strings.Builder
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return "", fmt.Errorf("exec failed: %w, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// updateStatus updates the OnionService status
func (r *OnionServiceReconciler) updateStatus(ctx context.Context, onion *v1beta1.OnionService, phase, onionAddress, message string) error {
	onion.Status.Phase = phase
	onion.Status.OnionAddress = onionAddress
	onion.Status.Message = message

	return r.Status().Update(ctx, onion)
}

// func (r *OnionServiceReconciler) cleanupOnionService(ctx context.Context, onion *v1beta1.OnionService) error {
// 	return nil
// }

func (r *OnionServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.OnionService{}).
		Complete(r)
}
