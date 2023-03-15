## About Contour

Contour is an [ingress controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) for Kubernetes that works by deploying the [Envoy proxy](https://www.envoyproxy.io/) as a reverse proxy and load balancer.
Contour supports dynamic configuration updates out of the box while maintaining a lightweight profile.

Contour supports multiple configuration APIs in order to meet the needs of as many users as possible:

- **[Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/)** - A stable upstream API that enables basic ingress use cases.
- **[HTTPProxy](https://projectcontour.io/docs/main/config/fundamentals/)** - Contour's Custom Resource Definition (CRD) which expands upon the functionality of the Ingress API to allow for a richer user experience as well as solve shortcomings in the original design.
- **[Gateway API](https://gateway-api.sigs.k8s.io/)** (beta) - A new CRD-based API managed by the [Kubernetes SIG-Network community](https://github.com/kubernetes/community/tree/master/sig-network) that aims to evolve Kubernetes service networking APIs in a vendor-neutral way.
