package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"
)

// Type holds this controller type
const Type = "Contour"

var _ rolloutsPlugin.TrafficRouterPlugin = (*RpcPlugin)(nil)

type RpcPlugin struct {
	IsTest               bool
	dynamicClient        dynamic.Interface
	UpdatedMockHTTPProxy *contourv1.HTTPProxy
}

type ContourTrafficRouting struct {
	// HTTPProxies is an array of strings which refer to the names of the HTTPProxies used to route
	// traffic to the service
	HTTPProxies []string `json:"httpProxies" protobuf:"bytes,1,name=httpProxies"`
}

func (r *RpcPlugin) InitPlugin() pluginTypes.RpcError {
	if r.IsTest {
		return pluginTypes.RpcError{}
	}

	cfg, err := utils.NewKubeConfig()
	if err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	r.dynamicClient, err = dynamic.NewForConfig(cfg)
	if err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	if err := validateRolloutParameters(rollout); err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	ctr, err := getContourTrafficRouting(rollout)
	if err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	ctx := context.Background()

	for _, proxy := range ctr.HTTPProxies {
		slog.Debug("updating httpproxy", slog.String("name", proxy))

		if err := r.updateHTTPProxy(ctx, proxy, rollout, desiredWeight); err != nil {
			slog.Error("failed to update httpproxy", slog.String("name", proxy), slog.Any("err", err))
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}

		slog.Info("successfully updated httpproxy", slog.String("name", proxy))
	}

	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	if err := validateRolloutParameters(rollout); err != nil {
		return pluginTypes.NotVerified, pluginTypes.RpcError{ErrorString: err.Error()}
	}

	ctr, err := getContourTrafficRouting(rollout)
	if err != nil {
		return pluginTypes.NotVerified, pluginTypes.RpcError{ErrorString: err.Error()}
	}

	ctx := context.Background()

	for _, proxy := range ctr.HTTPProxies {
		slog.Debug("verifying httpproxy", slog.String("name", proxy))

		verified, err := r.verifyHTTPProxy(ctx, proxy, rollout, desiredWeight)
		if err != nil {
			slog.Error("failed to verify httpproxy", slog.String("name", proxy), slog.Any("err", err))
			return pluginTypes.NotVerified, pluginTypes.RpcError{ErrorString: err.Error()}
		}
		if !verified {
			return pluginTypes.NotVerified, pluginTypes.RpcError{}
		}

		slog.Info("successfully verified httpproxy", slog.String("name", proxy))
	}

	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}

func (r *RpcPlugin) getHTTPProxy(ctx context.Context, namespace string, name string) (*contourv1.HTTPProxy, error) {
	unstr, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var httpProxy contourv1.HTTPProxy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy); err != nil {
		return nil, err
	}
	return &httpProxy, nil
}

func (r *RpcPlugin) updateHTTPProxy(ctx context.Context, httpProxyName string, rollout *v1alpha1.Rollout, desiredWeight int32) error {
	httpProxy, err := r.getHTTPProxy(ctx, rollout.Namespace, httpProxyName)
	if err != nil {
		return err
	}

	canarySvc, stableSvc, err := getCanaryAndStableServices(httpProxy, rollout)
	if err != nil {
		return err
	}

	slog.Debug("old weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	canarySvc.Weight = int64(desiredWeight)
	stableSvc.Weight = 100 - canarySvc.Weight

	slog.Debug("new weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&httpProxy)
	if err != nil {
		return err
	}
	updated, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Update(ctx, &unstructured.Unstructured{Object: m}, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	if r.IsTest {
		var proxy contourv1.HTTPProxy
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(updated.UnstructuredContent(), &proxy); err != nil {
			return err
		}
		r.UpdatedMockHTTPProxy = &proxy
	}

	return nil
}

func (r *RpcPlugin) verifyHTTPProxy(ctx context.Context, httpProxyName string, rollout *v1alpha1.Rollout, desiredWeight int32) (bool, error) {
	httpProxy, err := r.getHTTPProxy(ctx, rollout.Namespace, httpProxyName)
	if err != nil {
		return false, err
	}

	validCondition := httpProxy.Status.GetConditionFor(contourv1.ValidConditionType)
	if validCondition == nil {
		slog.Debug("unable to find valid status condition", slog.String("name", httpProxyName))
		return false, nil
	}
	if validCondition.Status != metav1.ConditionTrue {
		slog.Debug(fmt.Sprintf("condition status is not %s", metav1.ConditionTrue), slog.String("name", httpProxyName))
		return false, nil
	}
	if validCondition.ObservedGeneration != httpProxy.Generation {
		slog.Debug("condition is out of date with respect to the current state of the instance", slog.String("name", httpProxyName))
		return false, nil
	}

	canarySvc, stableSvc, err := getCanaryAndStableServices(httpProxy, rollout)
	if err != nil {
		return false, err
	}

	canarySvcDesiredWeight := int64(desiredWeight)
	stableSvcDesiredWeight := 100 - canarySvcDesiredWeight
	if canarySvc.Weight != canarySvcDesiredWeight || stableSvc.Weight != stableSvcDesiredWeight {
		slog.Debug(fmt.Sprintf("expected weights are canary=%d and stable=%d, but got canary=%d and stable=%d", canarySvcDesiredWeight, stableSvcDesiredWeight, canarySvc.Weight, stableSvc.Weight), slog.String("name", httpProxyName))
		return false, nil
	}

	return true, nil
}

func getCanaryAndStableServices(httpProxy *contourv1.HTTPProxy, rollout *v1alpha1.Rollout) (*contourv1.Service, *contourv1.Service, error) {
	canarySvcName := rollout.Spec.Strategy.Canary.CanaryService
	stableSvcName := rollout.Spec.Strategy.Canary.StableService

	slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

	// TODO: filter by condition(s)
	svcMap := getServiceMap(httpProxy)

	canarySvc, err := getService(canarySvcName, svcMap)
	if err != nil {
		return nil, nil, err
	}

	stableSvc, err := getService(stableSvcName, svcMap)
	if err != nil {
		return nil, nil, err
	}

	return canarySvc, stableSvc, nil
}

func getContourTrafficRouting(rollout *v1alpha1.Rollout) (*ContourTrafficRouting, error) {
	var ctr ContourTrafficRouting
	if err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins["argoproj-labs/contour"], &ctr); err != nil {
		return nil, err
	}
	return &ctr, nil
}

func getService(name string, svcMap map[string]*contourv1.Service) (*contourv1.Service, error) {
	svc, ok := svcMap[name]
	if !ok {
		return nil, fmt.Errorf("the service: %s is not found in httpproxy", name)
	}
	return svc, nil
}

func getServiceMap(httpProxy *contourv1.HTTPProxy) map[string]*contourv1.Service {
	svcMap := make(map[string]*contourv1.Service)
	for _, r := range httpProxy.Spec.Routes {
		for i := range r.Services {
			s := &r.Services[i]
			svcMap[s.Name] = s
		}
	}
	return svcMap
}

func validateRolloutParameters(rollout *v1alpha1.Rollout) error {
	if rollout == nil || rollout.Spec.Strategy.Canary == nil || rollout.Spec.Strategy.Canary.StableService == "" || rollout.Spec.Strategy.Canary.CanaryService == "" {
		return fmt.Errorf("illegal parameter(s)")
	}
	return nil
}
