## About Contour

Contour is an [ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) for Kubernetes that works by deploying the [Envoy proxy](https://www.envoyproxy.io/) as a reverse proxy and load balancer.
Contour supports dynamic configuration updates out of the box while maintaining a lightweight profile.

Contour supports multiple configuration APIs in order to meet the needs of as many users as possible:

- **[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/)** - A stable upstream API that enables basic ingress use cases.
- **[HTTPProxy](https://projectcontour.io/docs/main/config/fundamentals/)** - Contour's Custom Resource Definition (CRD) which expands upon the functionality of the Ingress API to allow for a richer user experience as well as solve shortcomings in the original design.
- **[Gateway API](https://gateway-api.sigs.k8s.io/)** (beta) - A new CRD-based API managed by the [Kubernetes SIG-Network community](https://github.com/kubernetes/community/tree/master/sig-network) that aims to evolve Kubernetes service networking APIs in a vendor-neutral way.

## How to integrate Contour with Argo Rollouts

NOTES:

**_1. The files as follows (and the codes in it) just for illustrative purposes only, please do not use directly!!!_**

**_2. The argo-rollouts >= [v1.5.0-rc1](https://github.com/argoproj/argo-rollouts/releases/tag/v1.5.0-rc1)_**

Steps:

1. Run the `yaml/rbac.yaml` to add the role for operate on the `HTTPProxy`.
2. Build this plugin.
3. Put the plugin somewhere & mount on to the `argo-rollouts`container (Please refer to the example YAML below to modify the deployment):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: argo-rollouts
  namespace: argo-rollouts
spec:
  template:
    spec:
      ...
      volumes:
        ...
         - name: contour-plugin
           hostPath:
             path: /CHANGE-ME/rollouts-plugin-trafficrouter-contour
             type: ''
      containers:
        - name: argo-rollouts
        ...
          volumeMounts:
             - name: contour-plugin
               mountPath: /CHANGE-ME/rollouts-plugin-trafficrouter-contour

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
      location: "file://CHANGE-ME/rollouts-plugin-trafficrouter-contour"
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
  replicas: 2
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
            httpProxies:
              - rollouts-demo
            namespace: rollouts-demo
  workloadRef:
    apiVersion: apps/v1
    kind: Deployment
    name: canary
```

6. Enjoy It.

## Contributing

Thanks for taking the time to join our community and start contributing!

- Please familiarize yourself with the [Code of Conduct](/CODE_OF_CONDUCT.md) before contributing.
- Check out the [open issues](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/issues).