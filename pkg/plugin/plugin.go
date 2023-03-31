package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	pluginTypes "github.com/argoproj/argo-rollouts/utils/plugin/types"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"golang.org/x/exp/slog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Type holds this controller type
const Type = "Contour"

func getKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, pluginTypes.RpcError{ErrorString: err.Error()}
	}
	return config, nil
}

type RpcPlugin struct {
	IsTest        bool
	dynamicClient dynamic.Interface
}

type ContourTrafficRouting struct {
	// HTTPProxy refers to the name of the HTTPProxy used to route traffic to the
	// service
	HTTPProxy string `json:"httpProxy" protobuf:"bytes,1,name=httpProxy"`
	Namespace string `json:"namespace" protobuf:"bytes,2,name=namespace"`
	Stable    string `json:"stable" protobuf:"bytes,3,name=stable"`
	Canary    string `json:"canary" protobuf:"bytes,4,name=canary"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func must1[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

func (r *RpcPlugin) InitPlugin() (re pluginTypes.RpcError) {
	defer func() {
		if e := recover(); e != nil {
			re.ErrorString = e.(error).Error()
		}
	}()

	//TODO:
	if r.IsTest {
		//r.dynamicClient = must1(dynamic.NewForConfig(cfg))
	} else {
		cfg := must1(getKubeConfig())
		r.dynamicClient = must1(dynamic.NewForConfig(cfg))
	}

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

	ctx := context.Background()

	ctr := ContourTrafficRouting{}
	must(json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins["argoproj-labs/contour"], &ctr))

	slog.Debug("the plugin config",
		slog.String("ns", ctr.Namespace),
		slog.String("httpproxy", ctr.HTTPProxy),
		slog.Any("weight", desiredWeight))

	var httpProxy contourv1.HTTPProxy
	unstr := must1(r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(ctr.Namespace).Get(ctx, ctr.HTTPProxy, metav1.GetOptions{}))
	must(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy))

	canarySvcName := ctr.Canary
	stableSvcName := ctr.Stable

	slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

	// TODO: filter by condition(s)
	services := must1(getServiceList(httpProxy.Spec.Routes))
	canarySvc := must1(getService(canarySvcName, services))
	stableSvc := must1(getService(stableSvcName, services))

	canarySvc.Weight = int64(desiredWeight)
	stableSvc.Weight = 100 - canarySvc.Weight

	slog.Debug("new weight", slog.Int64("canary", canarySvc.Weight), slog.Int64("stable", stableSvc.Weight))

	m := must1(runtime.DefaultUnstructuredConverter.ToUnstructured(&httpProxy))
	_, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(ctr.Namespace).Update(ctx, &unstructured.Unstructured{Object: m}, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("update HTTPProxy is failed", slog.String("name", httpProxy.Name), slog.Any("err", err))
		must(err)
	}

	slog.Debug("update HTTPProxy is successfully")
	return
}

func getService(name string, services []contourv1.Service) (*contourv1.Service, error) {
	var selected *contourv1.Service
	for i := 0; i < len(services); i++ {

		svc := &services[i]
		if svc.Name == name {
			selected = svc
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("the service: %s is not found in HTTPProxy", name)
	}
	return selected, nil
}

func getServiceList(routes []contourv1.Route) ([]contourv1.Service, error) {
	for _, r := range routes {
		if r.Services == nil {
			continue
		}
		return r.Services, nil
	}
	return nil, errors.New("the services are not found in HTTPProxy")
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
