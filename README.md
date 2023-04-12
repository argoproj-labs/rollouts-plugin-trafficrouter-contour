## About Contour

Contour is an [ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) for Kubernetes that works by deploying the [Envoy proxy](https://www.envoyproxy.io/) as a reverse proxy and load balancer.
Contour supports dynamic configuration updates out of the box while maintaining a lightweight profile.

Contour supports multiple configuration APIs in order to meet the needs of as many users as possible:

- **[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/)** - A stable upstream API that enables basic ingress use cases.
- **[HTTPProxy](https://projectcontour.io/docs/main/config/fundamentals/)** - Contour's Custom Resource Definition (CRD) which expands upon the functionality of the Ingress API to allow for a richer user experience as well as solve shortcomings in the original design.
- **[Gateway API](https://gateway-api.sigs.k8s.io/)** (beta) - A new CRD-based API managed by the [Kubernetes SIG-Network community](https://github.com/kubernetes/community/tree/master/sig-network) that aims to evolve Kubernetes service networking APIs in a vendor-neutral way.

## How to integrate Contour with Argo Rollouts
NOTES:

***1. The file as follows just for illustrative purposes only, please do not use directly!!!***

***2. The argo-rollouts >= [v1.5.0-rc1](https://github.com/argoproj/argo-rollouts/releases/tag/v1.5.0-rc1)***

Steps:

1. Run the `yaml/rbac.yaml` to add the role for operate on the `HTTPProxy`.
2. Build this plugin.
3. Put the plugin somewhere & mount on to the container for `argo-rollouts`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
name: argo-rollouts
namespace: argo-rollouts
spec:
template:
   spec:
   volumes:
      - name: contour-plugin
      hostPath:
          path: /CHANGE-ME/rollouts-trafficrouter-contour-plugin
          type: ''
   containers:
      - name: argo-rollouts
      volumeMounts:
          - name: contour-plugin
          mountPath: /CHANGE-ME/rollouts-trafficrouter-contour-plugin
      
```

4. Create a ConfigMap to let `argo-rollouts` know the plugin's location:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
name: argo-rollouts-config
namespace: argo-rollouts
data:
trafficRouterPlugins: |-
   - name: "argoproj-labs/contour"
   location: "file://CHANGE-ME/rollouts-trafficrouter-contour-plugin/contour-plugin"
binaryData: {}

```

5. Create the `CR/Rollout` and put it into the operated services` namespace:
    
```yaml
apiVersion: argoproj.io/v1alpha1
   kind: Rollout
   metadata:
   name: rollouts-demo
   namespace: rollouts-demo
   spec:
   replicas: 6
   selector:
       matchLabels:
       app.kubernetes.io/instance: rollouts-demo
   strategy:
       canary:
       canaryService: canaryService
       stableService: stableService
       steps:
           - setWeight: 30
           - pause:
               duration: 10
       trafficRouting:
           plugins:
           argoproj-labs/contour:
               httpProxy: rollouts-demo
               namespace: rollouts-demo
   workloadRef:
       apiVersion: apps/v1
       kind: Deployment
       name: canary

```
    
6. Enjoy It.


## TODO: Contribution
