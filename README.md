## About Contour

Contour is an [ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) for Kubernetes that works by deploying the [Envoy proxy](https://www.envoyproxy.io/) as a reverse proxy and load balancer.
Contour supports dynamic configuration updates out of the box while maintaining a lightweight profile.

Contour supports multiple configuration APIs in order to meet the needs of as many users as possible:

- **[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/)** - A stable upstream API that enables basic ingress use cases.
- **[HTTPProxy](https://projectcontour.io/docs/main/config/fundamentals/)** - Contour's Custom Resource Definition (CRD) which expands upon the functionality of the Ingress API to allow for a richer user experience as well as solve shortcomings in the original design.
- **[Gateway API](https://gateway-api.sigs.k8s.io/)** (beta) - A new CRD-based API managed by the [Kubernetes SIG-Network community](https://github.com/kubernetes/community/tree/master/sig-network) that aims to evolve Kubernetes service networking APIs in a vendor-neutral way.

## How to integrate Contour with Argo Rollouts

### Install Rollouts Using Helm

Add the following code to your `valuse.yaml` file when install the argo-rollouts by helm:

```yaml
controller:
    initContainers:                                   
      - name: copy-contour-plugin
        image: release.daocloud.io/skoala/rollouts-plugin-trafficrouter-contour:v0.3.0
        command: ["/bin/sh", "-c"]                    
        args:
          - cp /bin/rollouts-plugin-trafficrouter-contour /plugins
        volumeMounts:                                 
          - name: contour-plugin
            mountPath: /plugins
    trafficRouterPlugins:                             
      trafficRouterPlugins: |-
        - name: argoproj-labs/contour
          location: "file:///plugins/rollouts-plugin-trafficrouter-contour"  
    volumes:                                           
      - name: contour-plugin
        emptyDir: {}
    volumeMounts:                                      
      - name: contour-plugin
        mountPath: /plugins
```

if argo-rollouts helm chart version >= [2.32.6], just set the `providerRBAC.contour` to `true` in the `values.yaml` file. Otherwise, you need to follow the steps below to create RBAC for operate on the `HTTPProxy`:

```bash
kubectl apply -f https://raw.githubusercontent.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/main/yaml/rbac.yaml

or

kubectl patch clusterrole argo-rollouts --type='json' -p='[{"op": "add", "path": "/rules/-", "value": {"apiGroups":["projectcontour.io"],"resources":["httpproxies"],"verbs":["get","list","watch","update","patch","delete"]}}]'
```

### Stand-alone installation

NOTES:

**_1. The file as follows (and the codes in it) just for illustrative purposes only, please do not use directly!_**

**_2. The argo-rollouts >= [v1.5.0-rc1](https://github.com/argoproj/argo-rollouts/releases/tag/v1.5.0-rc1)_**

Steps:

1. Run the `yaml/rbac.yaml` to add the role for operate on the `HTTPProxy`.
2. Build this plugin.
3. Put the plugin somewhere & mount on to the `argo-rollouts`container (please refer to the example YAML below to modify the deployment):

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
  workloadRef:
    apiVersion: apps/v1
    kind: Deployment
    name: canary
```

6. Enjoy It.

## Use it by Docker image

From v0.2.3, you can use this plugin from a init container, the plugin artifact location in the image is:

```
/bin/rollouts-plugin-trafficrouter-contour
```

The docker image with its artifact both support amd64 and arm64.

## Contributing

Thanks for taking the time to join our community and start contributing!

- Please familiarize yourself with the [Code of Conduct](/CODE_OF_CONDUCT.md) before contributing.
- Check out the [open issues](https://github.com/argoproj-labs/rollouts-plugin-trafficrouter-contour/issues).