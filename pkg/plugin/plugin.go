package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"golang.org/x/exp/slog"
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
	// HTTPProxy refers to the name of the HTTPProxy used to route traffic to the
	// service
	HTTPProxy string `json:"httpProxy" protobuf:"bytes,1,name=httpProxy"`
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

	ctx := context.Background()

	ctr := ContourTrafficRouting{}
	utils.Must(json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins["argoproj-labs/contour"], &ctr))

	var httpProxy contourv1.HTTPProxy
	unstr := utils.Must1(r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Get(ctx, ctr.HTTPProxy, metav1.GetOptions{}))
	utils.Must(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy))

	canarySvcName := rollout.Spec.Strategy.Canary.CanaryService
	stableSvcName := rollout.Spec.Strategy.Canary.StableService

	slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

	// TODO: filter by condition(s)
	canarySvc := utils.Must1(getService(canarySvcName, &httpProxy))
	stableSvc := utils.Must1(getService(stableSvcName, &httpProxy))

	slog.Debug("old weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	canarySvc.Weight = int64(desiredWeight)
	stableSvc.Weight = 100 - canarySvc.Weight

	slog.Debug("new weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	m := utils.Must1(runtime.DefaultUnstructuredConverter.ToUnstructured(&httpProxy))
	updated, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Update(ctx, &unstructured.Unstructured{Object: m}, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("update the HTTPProxy is failed", slog.String("name", httpProxy.Name), slog.Any("err", err))
		utils.Must(err)
	}

	if r.IsTest {

		proxy := contourv1.HTTPProxy{}
		utils.Must(runtime.DefaultUnstructuredConverter.FromUnstructured(updated.UnstructuredContent(), &proxy))
		r.UpdatedMockHTTPProxy = &proxy
	}

	slog.Info("update HTTPProxy is successfully")
	return
}

func getService(name string, httpProxy *contourv1.HTTPProxy) (*contourv1.Service, error) {
	for _, r := range httpProxy.Spec.Routes {
		for i := range r.Services {
			s := &r.Services[i]
			if s.Name == name {
				return s, nil
			}
		}
	}
	return nil, fmt.Errorf("the service: %s is not found in HTTPProxy", name)
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
