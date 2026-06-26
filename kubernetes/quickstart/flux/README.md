# Flux Quickstart

Example Flux resources for syncing `kubernetes/quickstart` from this repository.

Update `ref.tag` to a released version for production.

```sh
kubectl apply -f kubernetes/quickstart/flux/gitrepository.yaml
kubectl apply -f kubernetes/quickstart/flux/kustomization.yaml
```
