package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	jsonpatch "github.com/evanphx/json-patch"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"
)

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

func (r *RpcPlugin) SetWeight(rollout *v1alpha1.Rollout, canaryWeightPercent int32, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	if err := validateRolloutParameters(rollout); err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	ctr, err := getContourTrafficRouting(rollout)
	if err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	ctx := context.Background()

	for _, proxy := range ctr.HTTPProxies {
		slog.Debug("updating httpproxy weight", slog.String("name", proxy))

		if err := r.updateHTTPProxy(ctx, proxy, rollout, canaryWeightPercent); err != nil {
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

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, canaryWeightPercent int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
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

		verified, err := r.verifyHTTPProxy(ctx, proxy, rollout, canaryWeightPercent)
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
		return nil, fmt.Errorf("failed to get the httpproxy: %w", err)
	}

	var httpProxy contourv1.HTTPProxy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy); err != nil {
		return nil, fmt.Errorf("failed to convert the httpproxy: %w", err)
	}
	return &httpProxy, nil
}

func (r *RpcPlugin) updateHTTPProxy(
	ctx context.Context,
	httpProxyName string,
	rollout *v1alpha1.Rollout,
	canaryWeightPercent int32) error {

	httpProxy, err := r.getHTTPProxy(ctx, rollout.Namespace, httpProxyName)
	if err != nil {
		return err
	}

	patchData, patchType, err := createPatch(httpProxy, rollout, canaryWeightPercent)
	if err != nil {
		return fmt.Errorf("failed to create patch : %w", err)
	}
	updated, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Patch(ctx, httpProxyName, patchType, patchData, metav1.PatchOptions{})
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

func createPatch(httpProxy *contourv1.HTTPProxy, rollout *v1alpha1.Rollout, canaryWeightPercent int32) ([]byte, types.PatchType, error) {
	oldData, err := json.Marshal(httpProxy.DeepCopy())
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal the current configuration: %w", err)
	}

	canarySvc, stableSvc, totalWeight, err := getRouteServices(httpProxy, rollout)
	if err != nil {
		return nil, types.MergePatchType, err
	}
	slog.Debug("old weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	canarySvc.Weight, stableSvc.Weight = utils.CalcWeight(totalWeight, float32(canaryWeightPercent))
	slog.Debug("new weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	newData, err := json.Marshal(httpProxy)
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal the current configuration: %w", err)
	}

	// now default use json merge patch.
	patch, err := jsonpatch.CreateMergePatch(oldData, newData)
	return patch, types.MergePatchType, err
}

func (r *RpcPlugin) verifyHTTPProxy(
	ctx context.Context,
	httpProxyName string,
	rollout *v1alpha1.Rollout,
	canaryWeightPercent int32) (bool, error) {

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

	canarySvc, stableSvc, totalWeight, err := getRouteServices(httpProxy, rollout)
	if err != nil {
		return false, err
	}

	canaryWeight, stableWeight := utils.CalcWeight(totalWeight, float32(canaryWeightPercent))
	if canarySvc.Weight != canaryWeight || stableSvc.Weight != stableWeight {
		slog.Debug(fmt.Sprintf("expected weights are canary=%d and stable=%d, but got canary=%d and stable=%d", canaryWeight, stableWeight, canarySvc.Weight, stableSvc.Weight), slog.String("name", httpProxyName))
		return false, nil
	}

	return true, nil
}

func getRouteServices(httpProxy *contourv1.HTTPProxy, rollout *v1alpha1.Rollout) (
	*contourv1.Service, *contourv1.Service, int64, error) {
	canarySvcName := rollout.Spec.Strategy.Canary.CanaryService
	stableSvcName := rollout.Spec.Strategy.Canary.StableService

	slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

	svcMap := getServiceMap(httpProxy, canarySvcName)

	canarySvc, err := getService(canarySvcName, svcMap)
	if err != nil {
		return nil, nil, 0, err
	}

	stableSvc, err := getService(stableSvcName, svcMap)
	if err != nil {
		return nil, nil, 0, err
	}

	otherWeight := int64(0)
	for name, svc := range svcMap {
		if name == stableSvcName || name == canarySvcName {
			continue
		}
		otherWeight += svc.Weight
	}

	// the total weight must equals to 100
	if otherWeight+canarySvc.Weight+stableSvc.Weight != 100 {
		return nil, nil, 0, fmt.Errorf("the total weight must equals to 100")
	}

	return canarySvc, stableSvc, 100 - otherWeight, nil
}

func getContourTrafficRouting(rollout *v1alpha1.Rollout) (*ContourTrafficRouting, error) {
	var ctr ContourTrafficRouting
	if err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[ConfigKey], &ctr); err != nil {
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

func getServiceMap(httpProxy *contourv1.HTTPProxy, canarySvcName string) map[string]*contourv1.Service {
	svcMap := make(map[string]*contourv1.Service)

	// filter the services by canary service name
	filter := func(services []contourv1.Service) bool {
		for _, svc := range services {
			if svc.Name == canarySvcName {
				return true
			}
		}
		return false
	}
	// TODO: same service in multi conditions
	for _, r := range httpProxy.Spec.Routes {
		if filter(r.Services) {
			for i := range r.Services {
				s := &r.Services[i]
				svcMap[s.Name] = s
			}
		}

	}
	return svcMap
}

func validateRolloutParameters(rollout *v1alpha1.Rollout) error {
	if rollout == nil || rollout.Spec.Strategy.Canary == nil || rollout.Spec.Strategy.Canary.StableService == "" || rollout.Spec.Strategy.Canary.CanaryService == "" {
		return fmt.Errorf("illegal parameter(s),both canary service and stable service must be specified")
	}
	return nil
}
