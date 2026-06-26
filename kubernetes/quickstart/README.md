# Quickstart Manifests

Kustomize bundle for installing the plugin in `cnpg-system`.

It includes:

- CNPG operator `INCLUDE_PLUGINS` ConfigMap
- cert-manager Issuer and Certificates for CNPG-I mTLS
- plugin RBAC, Deployment, and Service
- image override for `ghcr.io/odit-services/cnpg-plugin-pgdump`

Apply directly:

```sh
kubectl apply -k kubernetes/quickstart
kubectl -n cnpg-system rollout restart deployment/cnpg-controller-manager
kubectl -n cnpg-system rollout status deployment/cnpg-controller-manager
kubectl -n cnpg-system rollout status deployment/cnpg-plugin-pgdump
```

For production, pin `newTag` in `kustomization.yaml` to a released version instead of `main`.
