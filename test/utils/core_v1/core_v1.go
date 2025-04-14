package corev1

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Pod = func(opts ...func(any)) *corev1.Pod {
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

var WithOwnerReferences = func(or metav1.OwnerReference) func(any) {
	return func(o any) {
		o.(*corev1.Pod).OwnerReferences = append(o.(*corev1.Pod).OwnerReferences, or)
	}
}

var WithDeletionTimestamp = func() func(any) {
	return func(o any) {
		o.(client.Object).SetDeletionTimestamp(&metav1.Time{Time: time.Unix(0, 1)})
	}
}

var WithFinalizer = func(f string) func(any) {
	return func(o any) {
		o.(client.Object).SetFinalizers(append(o.(client.Object).GetFinalizers(), f))
	}
}
