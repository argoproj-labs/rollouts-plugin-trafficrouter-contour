package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// Type holds this controller type
const Type = "Contour"

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

func (r *RpcPlugin) InitPlugin() (re pluginTypes.RpcError) {
	defer func() {
		if e := recover(); e != nil {
			re.ErrorString = e.(error).Error()
		}
	}()

	if r.IsTest {
		return
	}

	cfg := utils.Must1(utils.NewKubeConfig())
	r.dynamicClient = utils.Must1(dynamic.NewForConfig(cfg))

	return
}
func (r *RpcPlugin) UpdateHash(rollout *v1alpha1.Rollout, canaryHash, stableHash string, additionalDestinations []v1alpha1.WeightDestination) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) mustUnmarshalTrafficRouting(rollout *v1alpha1.Rollout) *ContourTrafficRouting {
	ctr := &ContourTrafficRouting{}
	utils.Must(json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins["argoproj-labs/contour"], ctr))
	return ctr
}

func (r *RpcPlugin) mustHTTPProxy(ctx context.Context, namespace, name string) *contourv1.HTTPProxy {
	httpProxy := &contourv1.HTTPProxy{}
	unstr := utils.Must1(r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{}))
	utils.Must(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), httpProxy))
	return httpProxy
}

func (r *RpcPlugin) mustSetWeight(ctx context.Context, namespace, name, canarySvcName, stableSvcName string, desiredWeight int32) {
	slog.Debug("updating the HTTPProxy", slog.String("name", name))

	httpProxy := r.mustHTTPProxy(ctx, namespace, name)

	slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

	// TODO: filter by condition(s)
	svcMap := buildServiceMapFor(httpProxy)

	canarySvc := utils.Must1(serviceWithName(canarySvcName, svcMap))
	stableSvc := utils.Must1(serviceWithName(stableSvcName, svcMap))

	slog.Debug("old weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	canarySvc.Weight = int64(desiredWeight)
	stableSvc.Weight = 100 - canarySvc.Weight

	slog.Debug("new weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	m := utils.Must1(runtime.DefaultUnstructuredConverter.ToUnstructured(&httpProxy))
	updated, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(namespace).Update(ctx, &unstructured.Unstructured{Object: m}, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("update the HTTPProxy is failed", slog.String("name", name), slog.Any("err", err))
		utils.Must(err)
	}

	if r.IsTest {
		proxy := contourv1.HTTPProxy{}
		utils.Must(runtime.DefaultUnstructuredConverter.FromUnstructured(updated.UnstructuredContent(), &proxy))
		r.UpdatedMockHTTPProxy = &proxy
	}

	slog.Info("update the HTTPProxy is successfully", slog.String("name", name))
}
func (r *RpcPlugin) SetWeight(
	rollout *v1alpha1.Rollout,
	desiredWeight int32,
	additionalDestinations []v1alpha1.WeightDestination) (re pluginTypes.RpcError) {
	defer func() {
		if e := recover(); e != nil {
			re.ErrorString = e.(error).Error()
		}
	}()

	if rollout == nil || rollout.Spec.Strategy.Canary == nil ||
		rollout.Spec.Strategy.Canary.StableService == "" ||
		rollout.Spec.Strategy.Canary.CanaryService == "" {
		utils.Must(errors.New("illegal parameter(s)"))
	}

	canarySvcName := rollout.Spec.Strategy.Canary.CanaryService
	stableSvcName := rollout.Spec.Strategy.Canary.StableService

	ctr := r.mustUnmarshalTrafficRouting(rollout)
	for _, proxy := range ctr.HTTPProxies {
		r.mustSetWeight(context.Background(), rollout.Namespace, proxy, canarySvcName, stableSvcName, desiredWeight)
	}
	return
}

func serviceWithName(name string, svcMap map[string]*contourv1.Service) (*contourv1.Service, error) {
	svc, ok := svcMap[name]
	if !ok {
		return nil, fmt.Errorf("the service: %s is not found in HTTPProxy", name)
	}
	return svc, nil
}

func buildServiceMapFor(httpProxy *contourv1.HTTPProxy) map[string]*contourv1.Service {
	m := make(map[string]*contourv1.Service)
	for _, r := range httpProxy.Spec.Routes {
		for i := range r.Services {
			s := &r.Services[i]
			m[s.Name] = s
		}
	}
	return m
}

func (r *RpcPlugin) VerifyWeight(
	rollout *v1alpha1.Rollout,
	desiredWeight int32,
	additionalDestinations []v1alpha1.WeightDestination) (rv pluginTypes.RpcVerified, re pluginTypes.RpcError) {
	defer func() {
		if e := recover(); e != nil {
			re.ErrorString = e.(error).Error()
			rv = pluginTypes.NotVerified
		}
	}()

	if rollout == nil {
		utils.Must(errors.New("illegal parameter(s)"))
	}

	ctr := r.mustUnmarshalTrafficRouting(rollout)
	for _, proxy := range ctr.HTTPProxies {
		slog.Debug("verify the HTTPProxy", slog.String("name", proxy))

		httpProxy := r.mustHTTPProxy(context.Background(), rollout.Namespace, proxy)

		slog.Debug("the HTTPProxy status", slog.String("current", httpProxy.Status.CurrentStatus))

		if utils.ProxyStatus(httpProxy.Status.CurrentStatus) != utils.ProxyStatusValid {
			panic(fmt.Errorf("verify the HTTPProxy/%s's status is failed, desiredWeight: %d want: %s actual: %s", proxy, desiredWeight, utils.ProxyStatusValid, httpProxy.Status.CurrentStatus))
		}
	}

	slog.Info("verify weight is successfully", slog.Int64("desiredWeight", int64(desiredWeight)))

	rv = pluginTypes.Verified

	return
}

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) Type() string {
	return Type
}
