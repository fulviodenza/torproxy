package appsv1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1testutils "github.com/fulviodenza/torproxy/test/utils/core_v1"
)

var Deployment = func(opts ...func(any)) *appsv1.Deployment {
	p := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      corev1testutils.Pod().Name,
					Namespace: corev1testutils.Pod().Namespace,
					Labels:    corev1testutils.Pod().Labels,
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
