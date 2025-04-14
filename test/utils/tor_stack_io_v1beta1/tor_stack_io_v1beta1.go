package torstackiov1beta1

import (
	"github.com/fulviodenza/torproxy/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var OnionService = func(opts ...func(any)) *v1beta1.OnionService {
	t := &v1beta1.OnionService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: v1beta1.OnionServiceSpec{
			SOCKSPort: 9050,
		},
	}

	for _, f := range opts {
		f(t)
	}
	return t
}
