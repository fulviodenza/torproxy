package middleguardrelay

import (
	"context"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MiddleGuardRelayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=tor.stack.io,resources=
func (r MiddleGuardRelayReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := log.FromContext(ctx)
	log.Info("got new request: ", "namespace/name", req.NamespacedName.String())

	middleGuardRelay := &v1beta1.MiddleGuardRelay{}
	err := r.Get(ctx, req.NamespacedName, middleGuardRelay)
	if err != nil && !errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r MiddleGuardRelayReconciler) reconcilePod(middleGuardRelay v1beta1.MiddleGuardRelay) {
	// config := torrc.GenerateTorrcConfigForMiddleGuardRelay(&middleGuardRelay)
}

func (r MiddleGuardRelayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1beta1.MiddleGuardRelay{}).
		// When the pods managed by this configuration
		// are deleted or changed, reconcile the pod
		Owns(&corev1.Pod{}).
		Complete(r)
}
