package mocks

import (
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Namespace         = "default"
	StableServiceName = "argo-rollouts-stable-service"
	CanaryServiceName = "argo-rollouts-canary-service"
	HTTPProxyName     = "argo-rollouts-httpproxy"
)

var HTTPProxyObj = contourv1.HTTPProxy{
	ObjectMeta: metav1.ObjectMeta{
		Name:      HTTPProxyName,
		Namespace: Namespace,
	},
	Spec: contourv1.HTTPProxySpec{
		Routes: []contourv1.Route{
			{
				Services: []contourv1.Service{
					{
						Name:   StableServiceName,
						Weight: 100,
					},
					{
						Name: CanaryServiceName,
					},
				},
			},
		},
	},
}
