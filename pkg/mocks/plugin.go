package mocks

import (
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	StableServiceName     = "argo-rollouts-stable"
	CanaryServiceName     = "argo-rollouts-canary"
	AddOnServiceName      = "argo-rollouts-addon"
	AddOnRouteServiceName = "argo-rollouts-addon-route"

	HTTPProxyName               = "argo-rollouts"
	ValidHTTPProxyName          = "argo-rollouts-valid"
	OutdatedHTTPProxyName       = "argo-rollouts-outdated"
	InvalidHTTPProxyName        = "argo-rollouts-invalid"
	FalseConditionHTTPProxyName = "argo-rollouts-false-condition"

	// HTTPProxyAddOnWeight represents the add-ons services' weight in the total weight
	HTTPProxyAddOnWeight = 20

	// HTTPProxyCanaryWeightPercent represents the canary's weight for the canary deploment service (only)
	HTTPProxyCanaryWeightPercent = 40
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

func MakeName(origin string, appendPostfix ...bool) string {
	if len(appendPostfix) == 0 || !appendPostfix[0] {
		return origin
	}
	return origin + "-addon"
}

func MakeObjects(appendPostfix bool, addonServices ...contourv1.Service) []runtime.Object {
	httpProxy := newHTTPProxy(MakeName(HTTPProxyName, appendPostfix), addonServices...)
	validHttpProxy := newHTTPProxy(MakeName(ValidHTTPProxyName, appendPostfix), addonServices...)

	invalidHttpProxy := newHTTPProxy(MakeName(InvalidHTTPProxyName, appendPostfix), addonServices...)
	invalidHttpProxy.Status = contourv1.HTTPProxyStatus{
		Conditions: []contourv1.DetailedCondition{
			makeDetailedCondition(contourv1.ConditionTypeServiceError, contourv1.ConditionTrue),
		},
	}

	outdatedHttpProxy := newHTTPProxy(MakeName(OutdatedHTTPProxyName, appendPostfix), addonServices...)
	outdatedHttpProxy.Generation = httpProxyGeneration + 1

	falseConditionHttpProxy := newHTTPProxy(MakeName(FalseConditionHTTPProxyName, appendPostfix), addonServices...)
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

func mainServices(totalWeight int64) []contourv1.Service {
	canaryWeight, stableWeight := utils.CalcWeight(totalWeight, HTTPProxyCanaryWeightPercent)
	return []contourv1.Service{
		utils.MakeService(StableServiceName, stableWeight),
		utils.MakeService(CanaryServiceName, canaryWeight),
	}
}
func newHTTPProxy(name string, addOnServices ...contourv1.Service) *contourv1.HTTPProxy {
	totalWeight := int64(100)

	for _, svc := range addOnServices {
		totalWeight -= svc.Weight
	}

	services := mainServices(totalWeight)
	services = append(services, addOnServices...)
	canaryRoute := contourv1.Route{Services: services}

	addOnRoute := contourv1.Route{
		Services: []contourv1.Service{utils.MakeService(AddOnRouteServiceName, 100)},
	}

	httpproxy := &contourv1.HTTPProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: httpProxyGeneration,
		},
		Spec: contourv1.HTTPProxySpec{
			Routes: []contourv1.Route{canaryRoute, addOnRoute},
		},
		Status: contourv1.HTTPProxyStatus{
			Conditions: []contourv1.DetailedCondition{
				makeDetailedCondition(contourv1.ValidConditionType, contourv1.ConditionTrue),
			},
		},
	}
	return httpproxy
}
