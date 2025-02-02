package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fulviodenza/torproxy/api/v1beta1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
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

	torBridgeConfig := func(opts ...func(interface{})) *v1beta1.TorBridgeConfig {
		t := &v1beta1.TorBridgeConfig{
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

		for _, f := range opts {
			f(t)
		}
		return t
	}

	pod := func(opts ...func(interface{})) *corev1.Pod {
		p := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-pod",
				Namespace: "default",
				Labels: map[string]string{
					"tor": "hide-me",
				},
			},
		}

		for _, f := range opts {
			f(p)
		}
		return p
	}

	deployment := func(opts ...func(interface{})) *appsv1.Deployment {
		p := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "controller",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pod().Name,
						Namespace: pod().Namespace,
						Labels:    pod().Labels,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Image: "example/hello",
							},
						},
					},
				},
			},
		}

		for _, f := range opts {
			f(p)
		}
		return p
	}

	withOwnerReferences := func(or metav1.OwnerReference) func(interface{}) {
		return func(o interface{}) {
			o.(*corev1.Pod).OwnerReferences = append(o.(*corev1.Pod).OwnerReferences, or)
		}
	}

	tests := []struct {
		name                   string
		existingObjects        []runtime.Object
		expectedPodCount       int
		expectedPodName        string
		expectedDeploymentName string
	}{
		{
			name:             "Reconcile deletes unhidden pod and creates new pod with tor configuration",
			existingObjects:  []runtime.Object{torBridgeConfig(), pod()},
			expectedPodCount: 1,
			expectedPodName:  "existing-pod-hidden-",
		},
		{
			name: "Reconcile with pod with ownerReferences",
			existingObjects: []runtime.Object{torBridgeConfig(), deployment(), pod(withOwnerReferences(
				metav1.OwnerReference{
					Name: "controller",
					Kind: "Deployment",
				},
			))},
			expectedDeploymentName: "controller",
			expectedPodName:        "existing-pod-hidden-",
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

			if tt.expectedPodCount != 0 {
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
				foundTorContainer := false
				for _, container := range newPod.Spec.Containers {
					if container.Name == TorContainerName {
						foundTorContainer = true
						break
					}
				}

				if !foundTorContainer {
					t.Errorf("Expected tor container %s not found in pod %s", TorContainerName, newPod.Name)
				}
			}

			deploymentList := &appsv1.DeploymentList{}
			if err := fakeClient.List(context.Background(), deploymentList, client.InNamespace("default")); err != nil {
				t.Fatalf("List pods failed: %v", err)
			}

			if len(deploymentList.Items) > 0 {
				newDeployment := deploymentList.Items[0]
				if !cmp.Equal(newDeployment.Spec.Template.Name, tt.expectedPodName, cmpopts.EquateEmpty()) {
					if after, found := strings.CutPrefix(newDeployment.Name, tt.expectedDeploymentName); !found {
						fmt.Println(after)
						t.Errorf("Unexpected deployment name: got %v, want prefix %v", newDeployment.Name, tt.expectedDeploymentName)
					}
				}
			}

			if len(deploymentList.Items) == 0 && tt.expectedDeploymentName != "" {
				t.Errorf("Expected deployment")
			}
		})
	}
}
