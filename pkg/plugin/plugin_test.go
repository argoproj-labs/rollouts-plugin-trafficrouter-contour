package plugin

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/mocks"
	"github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/pkg/utils"

	"github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsPlugin "github.com/argoproj/argo-rollouts/rollout/trafficrouting/plugin/rpc"
	"github.com/argoproj/argo-rollouts/utils/plugin/types"
	goPlugin "github.com/hashicorp/go-plugin"
	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	fakeDynClient "k8s.io/client-go/dynamic/fake"
)

var testHandshake = goPlugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARGO_ROLLOUTS_RPC_PLUGIN",
	MagicCookieValue: "trafficrouter",
}

func TestRunSuccessfully(t *testing.T) {
	utils.InitLogger(slog.LevelDebug)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := runtime.NewScheme()
	b := runtime.SchemeBuilder{
		contourv1.AddToScheme,
	}

	_ = b.AddToScheme(s)

	// no add-on service
	objects := mocks.MakeObjects(false)

	addOnSvc := utils.MakeService(mocks.AddOnServiceName, mocks.HTTPProxyAddOnWeight)

	// with add-on service
	objects = append(objects, mocks.MakeObjects(true, addOnSvc)...)

	dynClient := fakeDynClient.NewSimpleDynamicClient(s, objects...)
	rpcPluginImp := &RpcPlugin{
		IsTest:        true,
		dynamicClient: dynClient,
	}

	// pluginMap is the map of plugins we can dispense.
	var pluginMap = map[string]goPlugin.Plugin{
		"RpcTrafficRouterPlugin": &rolloutsPlugin.RpcTrafficRouterPlugin{Impl: rpcPluginImp},
	}

	ch := make(chan *goPlugin.ReattachConfig, 1)
	closeCh := make(chan struct{})
	go goPlugin.Serve(&goPlugin.ServeConfig{
		HandshakeConfig: testHandshake,
		Plugins:         pluginMap,
		Test: &goPlugin.ServeTestConfig{
			Context:          ctx,
			ReattachConfigCh: ch,
			CloseCh:          closeCh,
		},
	})

	// We should get a config
	var config *goPlugin.ReattachConfig
	select {
	case config = <-ch:
	case <-time.After(2000 * time.Millisecond):
		t.Fatal("should've received reattach")
	}
	if config == nil {
		t.Fatal("config should not be nil")
	}

	// Connect!
	c := goPlugin.NewClient(&goPlugin.ClientConfig{
		Cmd:             nil,
		HandshakeConfig: testHandshake,
		Plugins:         pluginMap,
		Reattach:        config,
	})
	client, err := c.Client()
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	// Pinging should work
	if err := client.Ping(); err != nil {
		t.Fatalf("should not err: %s", err)
	}

	// Kill which should do nothing
	c.Kill()
	if err := client.Ping(); err != nil {
		t.Fatalf("should not err: %s", err)
	}

	// Request the plugin
	raw, err := client.Dispense("RpcTrafficRouterPlugin")
	if err != nil {
		t.Fail()
	}

	pluginInstance := raw.(*rolloutsPlugin.TrafficRouterPluginRPC)
	if err := pluginInstance.InitPlugin(); err.HasError() {
		t.Fail()
	}

	makeSetWeightTester := func(name string, totalWeight, canaryWeightPercent int32) func(t *testing.T) {
		return func(t *testing.T) {
			rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, name)

			if err := pluginInstance.SetWeight(rollout, canaryWeightPercent, []v1alpha1.WeightDestination{}); err.HasError() {
				t.FailNow()
			}

			canaryWeight, stableWeight := utils.CalcWeight(int64(totalWeight), float32(canaryWeightPercent))
			if rpcPluginImp.UpdatedMockHTTPProxy == nil {
				t.FailNow()
			}
			if len(rpcPluginImp.UpdatedMockHTTPProxy.Spec.Routes) == 0 {
				t.FailNow()
			}
			if len(rpcPluginImp.UpdatedMockHTTPProxy.Spec.Routes[0].Services) < 2 {
				t.FailNow()
			}

			svcs := rpcPluginImp.UpdatedMockHTTPProxy.Spec.Routes[0].Services
			if stableWeight != svcs[0].Weight {
				t.FailNow()
			}
			if canaryWeight != svcs[1].Weight {
				t.FailNow()
			}
		}
	}

	makeVerifyWeightTester := func(canaryWeightPercent int32, appendPostfix ...bool) func(t *testing.T) {
		return func(t *testing.T) {
			verifyWeight := func(httpProxyName string, canaryWeightPercent int32, expected types.RpcVerified) {
				rollout := newRollout(mocks.StableServiceName, mocks.CanaryServiceName, httpProxyName)

				actual, err := pluginInstance.VerifyWeight(rollout, canaryWeightPercent, []v1alpha1.WeightDestination{})
				if err.HasError() {
					t.Fail()
				}
				if actual != expected {
					t.Fail()
				}
			}

			verifyWeight(mocks.MakeName(mocks.ValidHTTPProxyName, appendPostfix...), canaryWeightPercent, types.Verified)
			verifyWeight(mocks.MakeName(mocks.ValidHTTPProxyName, appendPostfix...), canaryWeightPercent+10, types.NotVerified)
			verifyWeight(mocks.MakeName(mocks.InvalidHTTPProxyName, appendPostfix...), canaryWeightPercent, types.NotVerified)
			verifyWeight(mocks.MakeName(mocks.OutdatedHTTPProxyName, appendPostfix...), canaryWeightPercent, types.NotVerified)
			verifyWeight(mocks.MakeName(mocks.FalseConditionHTTPProxyName, appendPostfix...), canaryWeightPercent, types.NotVerified)
		}
	}

	t.Run(
		mocks.MakeName("SetWeight"),
		makeSetWeightTester(
			mocks.HTTPProxyName,
			100,
			mocks.HTTPProxyCanaryWeightPercent))
	t.Run(
		mocks.MakeName("SetWeight", true),
		makeSetWeightTester(
			mocks.MakeName(mocks.HTTPProxyName, true),
			100-mocks.HTTPProxyAddOnWeight,
			mocks.HTTPProxyCanaryWeightPercent))
	t.Run("VerifyWeight",
		makeVerifyWeightTester(mocks.HTTPProxyCanaryWeightPercent))
	t.Run(
		mocks.MakeName("VerifyWeight", true),
		makeVerifyWeightTester(mocks.HTTPProxyCanaryWeightPercent, true))

	// Canceling should cause an exit
	cancel()
	<-closeCh
}

func newRollout(stableSvc, canarySvc, httpProxyName string) *v1alpha1.Rollout {
	contourConfig := ContourTrafficRouting{
		HTTPProxies: []string{httpProxyName},
	}
	encodedContourConfig, err := json.Marshal(contourConfig)
	if err != nil {
		slog.Error("marshal the contour's config is failed", slog.Any("err", err))
		os.Exit(1)
	}

	return &v1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout",
			Namespace: "default",
		},
		Spec: v1alpha1.RolloutSpec{
			Strategy: v1alpha1.RolloutStrategy{
				Canary: &v1alpha1.CanaryStrategy{
					StableService: stableSvc,
					CanaryService: canarySvc,
					TrafficRouting: &v1alpha1.RolloutTrafficRouting{
						Plugins: map[string]json.RawMessage{
							"argoproj-labs/contour": encodedContourConfig,
						},
					},
				},
			},
		},
	}
}

func Test_createPatch(t *testing.T) {
	type args struct {
		httpProxy     *contourv1.HTTPProxy
		rollout       *v1alpha1.Rollout
		desiredWeight int32
	}
	tests := []struct {
		name          string
		args          args
		want          []byte
		wantPatchType k8stypes.PatchType
		wantErr       bool
	}{
		{
			name: "test create http proxy patch",
			args: args{
				httpProxy: &contourv1.HTTPProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: mocks.HTTPProxyName,
					},
					Spec: contourv1.HTTPProxySpec{
						Routes: []contourv1.Route{
							{
								Services: []contourv1.Service{
									{
										Name:   mocks.StableServiceName,
										Weight: 70,
									},
									{
										Name:   mocks.CanaryServiceName,
										Weight: 20,
									},
									{
										Name:   "others-service",
										Weight: 10,
									},
								},
							},
						},
					},
				},
				rollout: &v1alpha1.Rollout{
					Spec: v1alpha1.RolloutSpec{
						Strategy: v1alpha1.RolloutStrategy{
							Canary: &v1alpha1.CanaryStrategy{
								StableService: mocks.StableServiceName,
								CanaryService: mocks.CanaryServiceName,
							},
						},
					},
				},
				desiredWeight: 50,
			},
			want:          []byte(`{"spec":{"routes":[{"services":[{"name":"argo-rollouts-stable","port":0,"weight":45},{"name":"argo-rollouts-canary","port":0,"weight":45},{"name":"others-service","port":0,"weight":10}]}]}}`),
			wantPatchType: k8stypes.MergePatchType,
			wantErr:       false,
		},
		{
			name: "test create http proxy patch",
			args: args{
				httpProxy: &contourv1.HTTPProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name: mocks.HTTPProxyName,
					},
					Spec: contourv1.HTTPProxySpec{
						Routes: []contourv1.Route{
							{
								Services: []contourv1.Service{
									{
										Name:   mocks.StableServiceName,
										Weight: 70,
									},
									{
										Name:   mocks.CanaryServiceName,
										Weight: 20,
									},
									{
										Name:   "others-service",
										Weight: 10,
									},
								},
							},
							{
								Services: []contourv1.Service{
									{
										Name:   mocks.StableServiceName,
										Weight: 70,
									},
									{
										Name:   mocks.CanaryServiceName,
										Weight: 10,
									},
									{
										Name:   "others-service",
										Weight: 20,
									},
								},
							},
						},
					},
				},
				rollout: &v1alpha1.Rollout{
					Spec: v1alpha1.RolloutSpec{
						Strategy: v1alpha1.RolloutStrategy{
							Canary: &v1alpha1.CanaryStrategy{
								StableService: mocks.StableServiceName,
								CanaryService: mocks.CanaryServiceName,
							},
						},
					},
				},
				desiredWeight: 50,
			},
			want:          []byte(`{"spec":{"routes":[{"services":[{"name":"argo-rollouts-stable","port":0,"weight":45},{"name":"argo-rollouts-canary","port":0,"weight":45},{"name":"others-service","port":0,"weight":10}]},{"services":[{"name":"argo-rollouts-stable","port":0,"weight":40},{"name":"argo-rollouts-canary","port":0,"weight":40},{"name":"others-service","port":0,"weight":20}]}]}}`),
			wantPatchType: k8stypes.MergePatchType,
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotPatchType, err := createPatch(tt.args.httpProxy, tt.args.rollout, tt.args.desiredWeight)
			if (err != nil) != tt.wantErr {
				t.Errorf("createPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPatchType, tt.wantPatchType) {
				t.Errorf("createPatch() gotPatchType = %v, want %v", gotPatchType, tt.wantPatchType)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createPatch() got = %s, want %s", got, tt.want)
			}
		})
	}
}
