# Argo CD Quickstart

Example Argo CD `Application` for syncing `kubernetes/quickstart` from this repository.

Update `targetRevision` to a released version for production.

```sh
kubectl apply -f kubernetes/quickstart/argocd/application.yaml
```
