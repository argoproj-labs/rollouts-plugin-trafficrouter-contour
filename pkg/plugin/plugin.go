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
const pluginName = "argoproj-labs/contour"

var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("Rollout")

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

	for _, proxy := range ctr.HTTPProxies {
		slog.Debug("updating proxy", slog.String("proxy", proxy))

		var httpProxy contourv1.HTTPProxy
		unstr := utils.Must1(r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(rollout.Namespace).Get(ctx, proxy, metav1.GetOptions{}))
		utils.Must(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &httpProxy))

		canarySvcName := rollout.Spec.Strategy.Canary.CanaryService
		stableSvcName := rollout.Spec.Strategy.Canary.StableService

		slog.Debug("the services name", slog.String("stable", stableSvcName), slog.String("canary", canarySvcName))

		// TODO: filter by condition(s)
		svcMap := getServiceMap(&httpProxy)
		canarySvc := utils.Must1(getService(canarySvcName, svcMap))
		stableSvc := utils.Must1(getService(stableSvcName, svcMap))

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

		slog.Info("successfully updated HTTPProxy", slog.String("httpproxy", proxy))
	}
	return
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

func (r *RpcPlugin) SetHeaderRoute(rollout *v1alpha1.Rollout, headerRouting *v1alpha1.SetHeaderRoute) (re pluginTypes.RpcError) {
	if headerRouting == nil {
		return
	}
	defer func() {
		if e := recover(); e != nil {
			re.ErrorString = e.(error).Error()
		}
	}()

	if rollout == nil || rollout.Spec.Strategy.Canary == nil ||
		rollout.Spec.Strategy.Canary.CanaryService == "" ||
		rollout.Spec.Strategy.Canary.TrafficRouting == nil {
		utils.Must(errors.New("illegal parameter(s)"))
	}

	ctx := context.Background()

	ctr := ContourTrafficRouting{}
	utils.Must(json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[pluginName], &ctr))

	rootProxy, headerProxy, refProxy := r.findProxies(ctx, rollout, &ctr, headerRouting.Name)

	isNew := true
	if headerProxy != nil {
		if headerRouting.Match == nil {
			slog.Debug("remove the proxy for header", slog.String("name", headerProxy.Name))
			r.mustDeleteHTTPProxy(ctx, headerRouting.Name)
			r.mustExcludeHTTPProxy(ctx, rollout.Namespace, rootProxy, map[string]struct{}{headerProxy.Name: {}})
			return
		}
		isNew = false
	}

	// no root or no reference, skip it
	if rootProxy == nil || refProxy == nil {
		slog.Debug("the root or reference proxy is not existed")
		return
	}

	if isNew {
		headerProxy = makeHeaderProxy(rollout, headerProxy.Name, refProxy)
		r.mustIncludeHTTPProxy(ctx, rollout.Namespace, headerRouting.Name, rootProxy, refProxy)
	}
	mustSetMatchConditions(headerProxy, headerRouting)

	r.mustUpsertHTTPProxy(ctx, rollout.Namespace, headerProxy, isNew)
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) mustUpsertHTTPProxy(ctx context.Context, ns string, proxy *contourv1.HTTPProxy, isNew bool) {
	if isNew {
		r.mustCreateHTTPProxy(ctx, ns, proxy)
		return
	}

	r.mustUpdateHTTPProxy(ctx, ns, proxy)
}

func mustSetMatchConditions(proxy *contourv1.HTTPProxy, headerRouting *v1alpha1.SetHeaderRoute) {

	var conds []contourv1.MatchCondition

	for _, v := range proxy.Spec.Routes[0].Conditions {
		if v.Prefix != "" || v.QueryParameter != nil {
			tmp := v

			tmp.Header = nil
			conds = append(conds, tmp)
		}
	}

	for _, match := range headerRouting.Match {
		slog.Debug("add the header condition", slog.String("name", match.HeaderName))
		conds = append(proxy.Spec.Routes[0].Conditions, condition(&match))
	}
	proxy.Spec.Routes[0].Conditions = conds

}

func condition(match *v1alpha1.HeaderRoutingMatch) contourv1.MatchCondition {
	mc := &contourv1.HeaderMatchCondition{
		Name:  match.HeaderName,
		Exact: match.HeaderValue.Exact,
		Regex: match.HeaderValue.Regex,
	}

	if match.HeaderValue.Prefix != "" {
		mc.Regex = fmt.Sprintf("^%s.*", match.HeaderValue.Prefix)
	}

	return contourv1.MatchCondition{
		Header: mc,
	}
}

func (r *RpcPlugin) findProxies(
	ctx context.Context,
	rollout *v1alpha1.Rollout,
	ctr *ContourTrafficRouting,
	headerProxyName string) (root *contourv1.HTTPProxy, header *contourv1.HTTPProxy, ref *contourv1.HTTPProxy) {

	httpProxies := map[string]struct{}{}
	for _, name := range ctr.HTTPProxies {
		httpProxies[name] = struct{}{}
	}

listProxiesLoop:
	for _, proxy := range r.mustListHTTPProxies(ctx, rollout.Namespace).Items {
		if proxy.Name == headerProxyName {
			if !metav1.IsControlledBy(&proxy, rollout) {
				err := errors.New("duplicate httpproxy")
				slog.Error("list the http proxies is failed", slog.String("name", headerProxyName), slog.Any("err", err))
				utils.Must(err)
			}
			header = &proxy
			slog.Debug("the proxy for header is found", slog.String("name", headerProxyName))
		}

		if proxy.Spec.VirtualHost != nil {
			root = &proxy
			slog.Debug("the root proxy is found", slog.String("name", headerProxyName))
		}

		if _, ok := httpProxies[proxy.Name]; !ok {
			continue
		}

		if utils.ProxyStatus(proxy.Status.CurrentStatus) != utils.ProxyStatusValid {
			continue
		}

		// find the (first) http proxies which be used as a reference for create the 'header routing' http proxy
		for _, route := range proxy.Spec.Routes {
			for _, svc := range route.Services {
				if svc.Name == rollout.Spec.Strategy.Canary.CanaryService {
					slog.Debug("the reference proxy is found", slog.String("name", headerProxyName))
					ref = &proxy
					break listProxiesLoop
				}
			}
		}
	}

	return
}
func makeHeaderProxy(rollout *v1alpha1.Rollout, name string, refProxy *contourv1.HTTPProxy) *contourv1.HTTPProxy {
	p := &contourv1.HTTPProxy{
		TypeMeta: refProxy.DeepCopy().TypeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rollout.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rollout, controllerKind),
			},
		},
	}

loop:
	for _, route := range refProxy.Spec.Routes {
		for _, svc := range route.Services {
			if svc.Name == rollout.Spec.Strategy.Canary.CanaryService {
				tmp := route.DeepCopy()
				tmp.Services = []contourv1.Service{*svc.DeepCopy()}
				p.Spec.Routes = []contourv1.Route{*tmp}
				slog.Debug("the reference route is found")
				break loop
			}
		}
	}

	return p
}

func (r *RpcPlugin) SetMirrorRoute(rollout *v1alpha1.Rollout, setMirrorRoute *v1alpha1.SetMirrorRoute) pluginTypes.RpcError {
	return pluginTypes.RpcError{}
}

func (r *RpcPlugin) VerifyWeight(rollout *v1alpha1.Rollout, desiredWeight int32, additionalDestinations []v1alpha1.WeightDestination) (pluginTypes.RpcVerified, pluginTypes.RpcError) {
	return pluginTypes.Verified, pluginTypes.RpcError{}
}

func (r *RpcPlugin) RemoveManagedRoutes(rollout *v1alpha1.Rollout) (re pluginTypes.RpcError) {
	defer func() {
		if e := recover(); e != nil {
			re.ErrorString = e.(error).Error()
		}
	}()

	if rollout == nil || rollout.Spec.Strategy.Canary == nil ||
		rollout.Spec.Strategy.Canary.CanaryService == "" ||
		rollout.Spec.Strategy.Canary.TrafficRouting == nil {
		utils.Must(errors.New("illegal parameter(s)"))
	}

	managedRoutes := rollout.Spec.Strategy.Canary.TrafficRouting.ManagedRoutes
	if len(managedRoutes) == 0 {
		slog.Debug("no managed routes")
		return
	}

	ctx := context.Background()

	ctr := ContourTrafficRouting{}
	utils.Must(json.Unmarshal(rollout.Spec.Strategy.Canary.TrafficRouting.Plugins[pluginName], &ctr))

	managedRouteNames := map[string]struct{}{}
	for _, item := range managedRoutes {
		managedRouteNames[item.Name] = struct{}{}
	}

	var rootProxy *contourv1.HTTPProxy
	for _, item := range r.mustListHTTPProxies(ctx, rollout.Namespace).Items {
		if item.Spec.VirtualHost != nil {
			rootProxy = &item
			continue
		}
		if _, ok := managedRouteNames[item.Name]; ok && metav1.IsControlledBy(&item, rollout) {
			r.mustDeleteHTTPProxy(ctx, item.Name)
		}
	}

	if rootProxy == nil {
		return
	}

	r.mustExcludeHTTPProxy(ctx, rollout.Namespace, rootProxy, managedRouteNames)
	return
}

func (r *RpcPlugin) mustDeleteHTTPProxy(ctx context.Context, name string) {
	utils.Must(r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Delete(ctx, name, metav1.DeleteOptions{}))
	slog.Debug("delete httpproxy is successfully", slog.String("name", name))
}
func (r *RpcPlugin) mustExcludeHTTPProxy(
	ctx context.Context,
	ns string,
	root *contourv1.HTTPProxy,
	excludes map[string]struct{}) {

	if root == nil {
		return
	}

	var remains []contourv1.Include
	for _, v := range root.Spec.Includes {
		if _, ok := excludes[v.Name]; !ok {
			remains = append(remains, v)
		}
	}
	root.Spec.Includes = remains
	r.mustUpdateHTTPProxy(ctx, ns, root)
}

func (r *RpcPlugin) mustUpdateHTTPProxy(ctx context.Context, ns string, proxy *contourv1.HTTPProxy) {
	m := utils.Must1(runtime.DefaultUnstructuredConverter.ToUnstructured(proxy))
	_, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(ns).Update(ctx, &unstructured.Unstructured{Object: m}, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("update the proxy is failed", slog.String("name", proxy.Name), slog.Any("err", err))
	}
	utils.Must(err)
	slog.Debug("update httpproxy is succssfully", slog.String("name", proxy.Name))
}

func (r *RpcPlugin) mustCreateHTTPProxy(ctx context.Context, ns string, proxy *contourv1.HTTPProxy) {
	m := utils.Must1(runtime.DefaultUnstructuredConverter.ToUnstructured(proxy))
	_, err := r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(ns).Create(ctx, &unstructured.Unstructured{Object: m}, metav1.CreateOptions{})
	if err != nil {
		slog.Error("create the proxy is failed", slog.String("name", proxy.Name), slog.Any("err", err))
	}
	utils.Must(err)
	slog.Debug("create httpproxy is succssfully", slog.String("name", proxy.Name))
}

func (r *RpcPlugin) mustIncludeHTTPProxy(
	ctx context.Context,
	ns string,
	name string,
	root *contourv1.HTTPProxy,
	refProxy *contourv1.HTTPProxy) {

	if root == nil {
		return
	}

	include := contourv1.Include{
		Name:      name,
		Namespace: ns,
	}

	if refProxy != nil {
		for _, v := range root.Spec.Includes {
			if v.Name == refProxy.Name {
				include.Conditions = v.DeepCopy().Conditions
				break
			}
		}
	}

	root.Spec.Includes = append(root.Spec.Includes, include)
	slog.Debug("the header proxy is appended to root")
	r.mustUpdateHTTPProxy(ctx, ns, root)

}

func (r *RpcPlugin) mustListHTTPProxies(ctx context.Context, ns string) contourv1.HTTPProxyList {
	unstr := utils.Must1(r.dynamicClient.Resource(contourv1.HTTPProxyGVR).Namespace(ns).List(ctx, metav1.ListOptions{}))

	var proxies contourv1.HTTPProxyList
	utils.Must(runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &proxies))
	return proxies
}

func (r *RpcPlugin) Type() string {
	return Type
}
