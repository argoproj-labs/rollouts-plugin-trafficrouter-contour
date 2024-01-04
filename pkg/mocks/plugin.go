package mocks

import (
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	StableServiceName = "argo-rollouts-stable"
	CanaryServiceName = "argo-rollouts-canary"

	HTTPProxyName               = "argo-rollouts"
	ValidHTTPProxyName          = "argo-rollouts-valid"
	OutdatedHTTPProxyName       = "argo-rollouts-outdated"
	InvalidHTTPProxyName        = "argo-rollouts-invalid"
	FalseConditionHTTPProxyName = "argo-rollouts-false-condition"

	HTTPProxyDesiredWeight = 20
)

const (
	namespace           = "default"
	httpProxyGeneration = 1
)

func makeDetailedCondition(typ string, status contourv1.ConditionStatus) contourv1.DetailedCondition {
	return contourv1.DetailedCondition{
		Condition: contourv1.Condition{
			Type:               typ,
			Status:             status,
			ObservedGeneration: httpProxyGeneration,
		},
	}
}

func makeService(name string, weight int64) contourv1.Service {
	return contourv1.Service{
		Name:   name,
		Weight: weight,
	}
}
func MakeObjects() []runtime.Object {
	httpProxy := newHTTPProxy(HTTPProxyName)
	validHttpProxy := newHTTPProxy(ValidHTTPProxyName)

	invalidHttpProxy := newHTTPProxy(InvalidHTTPProxyName)
	invalidHttpProxy.Status = contourv1.HTTPProxyStatus{
		Conditions: []contourv1.DetailedCondition{
			makeDetailedCondition(contourv1.ConditionTypeServiceError, contourv1.ConditionTrue),
		},
	}

	outdatedHttpProxy := newHTTPProxy(OutdatedHTTPProxyName)
	outdatedHttpProxy.Generation = httpProxyGeneration + 1

	falseConditionHttpProxy := newHTTPProxy(FalseConditionHTTPProxyName)
	falseConditionHttpProxy.Status = contourv1.HTTPProxyStatus{
		Conditions: []contourv1.DetailedCondition{
			makeDetailedCondition(contourv1.ValidConditionType, contourv1.ConditionFalse),
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
			Namespace:  namespace,
			Generation: httpProxyGeneration,
		},
		Spec: contourv1.HTTPProxySpec{
			Routes: []contourv1.Route{
				{
					Services: []contourv1.Service{
						makeService(StableServiceName, 100-HTTPProxyDesiredWeight),
						makeService(CanaryServiceName, HTTPProxyDesiredWeight),
					},
				},
			},
		},
		Status: contourv1.HTTPProxyStatus{
			Conditions: []contourv1.DetailedCondition{
				makeDetailedCondition(contourv1.ValidConditionType, contourv1.ConditionTrue),
			},
		},
	}
}
