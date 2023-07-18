package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"golang.org/x/exp/slog"
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
	if rollout == nil || rollout.Spec.Strategy.Canary == nil || rollout.Spec.Strategy.Canary.StableService == "" || rollout.Spec.Strategy.Canary.CanaryService == "" {
		return pluginTypes.RpcError{ErrorString: "illegal parameter(s)"}
	}

	ctx := context.Background()

	ctr := ContourTrafficRouting{}
	if err := json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins["argoproj-labs/contour"], &ctr); err != nil {
		return pluginTypes.RpcError{ErrorString: err.Error()}
	}

	for _, proxy := range ctr.HTTPProxies {
		slog.Debug("updating proxy", slog.String("proxy", proxy))

		var httpProxy contourv1.HTTPProxy
		unstr, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Get(ctx, proxy, metav1.GetOptions{})
		if err != nil {
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy); err != nil {
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}

		canarySvcName := rollout.Spec.Strategy.Canary.CanaryService
		stableSvcName := rollout.Spec.Strategy.Canary.StableService

		slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

		// TODO: filter by condition(s)
		svcMap := getServiceMap(&httpProxy)

		canarySvc, err := getService(canarySvcName, svcMap)
		if err != nil {
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}

		stableSvc, err := getService(stableSvcName, svcMap)
		if err != nil {
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}

		slog.Debug("old weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

		canarySvc.Weight = int64(desiredWeight)
		stableSvc.Weight = 100 - canarySvc.Weight

		slog.Debug("new weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&httpProxy)
		if err != nil {
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}
		updated, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Update(ctx, &unstructured.Unstructured{Object: m}, metav1.UpdateOptions{})
		if err != nil {
			slog.Error("update the HTTPProxy is failed", slog.String("name", httpProxy.Name), slog.Any("err", err))
			return pluginTypes.RpcError{ErrorString: err.Error()}
		}

		if r.IsTest {
			proxy := contourv1.HTTPProxy{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(updated.UnstructuredContent(), &proxy); err != nil {
				return pluginTypes.RpcError{ErrorString: err.Error()}
			}
			r.UpdatedMockHTTPProxy = &proxy
		}

		slog.Info("successfully updated HTTPProxy", slog.String("httpproxy", proxy))
	}

	return pluginTypes.RpcError{}
}

func getService(name string, svcMap map[string]*contourv1.Service) (*contourv1.Service, error) {
	svc, ok := svcMap[name]
	if !ok {
		return nil, fmt.Errorf("the service: %s is not found in HTTPProxy", name)
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

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}
