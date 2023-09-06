package mocks

import (
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	Namespace         = "default"
	StableServiceName = "argo-rollouts-stable"
	CanaryServiceName = "argo-rollouts-canary"

	HTTPProxyName               = "argo-rollouts"
	ValidHTTPProxyName          = "argo-rollouts-valid"
	OutdatedHTTPProxyName       = "argo-rollouts-outdated"
	InvalidHTTPProxyName        = "argo-rollouts-invalid"
	FalseConditionHTTPProxyName = "argo-rollouts-false-condition"

	HTTPProxyGeneration    = 1
	HTTPProxyDesiredWeight = 20
)

func MakeObjects() []runtime.Object {
	httpProxy := newHTTPProxy(HTTPProxyName)

	validHttpProxy := newHTTPProxy(ValidHTTPProxyName)

	invalidHttpProxy := newHTTPProxy(InvalidHTTPProxyName)
	invalidHttpProxy.Status = contourv1.HTTPProxyStatus{
		Conditions: []contourv1.DetailedCondition{
			{
				Condition: contourv1.Condition{
					Type:               contourv1.ConditionTypeServiceError,
					Status:             contourv1.ConditionTrue,
					ObservedGeneration: HTTPProxyGeneration,
				},
			},
		},
	}

	outdatedHttpProxy := newHTTPProxy(OutdatedHTTPProxyName)
	outdatedHttpProxy.Generation = HTTPProxyGeneration + 1

	falseConditionHttpProxy := newHTTPProxy(FalseConditionHTTPProxyName)
	falseConditionHttpProxy.Status = contourv1.HTTPProxyStatus{
		Conditions: []contourv1.DetailedCondition{
			{
				Condition: contourv1.Condition{
					Type:               contourv1.ValidConditionType,
					Status:             contourv1.ConditionFalse,
					ObservedGeneration: HTTPProxyGeneration,
				},
			},
		},
	}

	objs := []runtime.Object{
		httpProxy,
		validHttpProxy,
		invalidHttpProxy,
		outdatedHttpProxy,
		falseConditionHttpProxy,
	}
	return objs
}

func newHTTPProxy(name string) *contourv1.HTTPProxy {
	return &contourv1.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  Namespace,
			Generation: HTTPProxyGeneration,
		},
		Spec: contourv1.HTTPProxySpec{
			Routes: []contourv1.Route{
				{
					Services: []contourv1.Service{
						{
							Name:   StableServiceName,
							Weight: 100 - HTTPProxyDesiredWeight,
						},
						{
							Name:   CanaryServiceName,
							Weight: HTTPProxyDesiredWeight,
						},
					},
				},
			},
		},
		Status: contourv1.HTTPProxyStatus{
			Conditions: []contourv1.DetailedCondition{
				{
					Condition: contourv1.Condition{
						Type:               contourv1.ValidConditionType,
						Status:             contourv1.ConditionTrue,
						ObservedGeneration: HTTPProxyGeneration,
					},
				},
			},
		},
	}
}
