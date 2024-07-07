package controller

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestTorBridgeConfigReconciler_Reconcile(t *testing.T) {
	s := scheme.Scheme
	if err := v1beta1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add v1beta1 scheme: %v", err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("Unable to add corev1 scheme: %v", err)
	}

	torBridgeConfig := &v1beta1.TorBridgeConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: v1beta1.TorBridgeConfigSpec{
			Image:     "tor-image",
			OrPort:    9001,
			DirPort:   9030,
			SOCKSPort: 9050,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "existing-pod",
			Namespace: "default",
			Labels: map[string]string{
				"tor": "hide-me",
			},
		},
	}

	tests := []struct {
		name                string
		existingObjects     []runtime.Object
		expectedPodCount    int
		expectedPodName     string
		expectedSidecarName string
	}{
		{
			name:                "Reconcile deletes unhidden pod and creates new pod with sidecar",
			existingObjects:     []runtime.Object{torBridgeConfig, pod},
			expectedPodCount:    1,
			expectedPodName:     "existing-pod-hidden-",
			expectedSidecarName: "tor-bridge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewFakeClient(tt.existingObjects...)

			reconciler := &TorBridgeConfigReconciler{
				Client: fakeClient,
				Scheme: s,
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-config",
					Namespace: "default",
				},
			}

			_, err := reconciler.Reconcile(context.Background(), req)
			if err != nil {
				t.Fatalf("Reconcile failed: %v", err)
			}

			podList := &corev1.PodList{}
			if err := fakeClient.List(context.Background(), podList, client.InNamespace("default")); err != nil {
				t.Fatalf("List pods failed: %v", err)
			}

			if len(podList.Items) != tt.expectedPodCount {
				t.Fatalf("Expected %d pod(s), but found %d", tt.expectedPodCount, len(podList.Items))
			}

			newPod := podList.Items[0]
			if !cmp.Equal(newPod.Name, tt.expectedPodName, cmpopts.EquateEmpty()) {
				if after, found := strings.CutPrefix(newPod.Name, tt.expectedPodName); !found {
					fmt.Println(after)
					t.Errorf("Unexpected pod name: got %v, want prefix %v", newPod.Name, tt.expectedPodName)
				}
			}

			foundSidecar := false
			for _, container := range newPod.Spec.Containers {
				if container.Name == tt.expectedSidecarName {
					foundSidecar = true
					break
				}
			}

			if !foundSidecar {
				t.Errorf("Expected sidecar container %s not found in pod %s", tt.expectedSidecarName, newPod.Name)
			}
		})
	}
}
