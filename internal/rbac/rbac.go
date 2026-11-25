// Package rbac lets the backup sidecar, running with the service account of the Cluster pods, read the
// secrets referenced by the backups.
package rbac

import (
	"context"
	"slices"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Name returns the name of the Role and RoleBinding of a Cluster.
func Name(clusterName string) string {
	return clusterName + "-pgdump-backup"
}

// EnsureSecretAccess grants the service account of the Cluster pods read access to the given secrets.
// Secrets already granted are kept, so that several backups of the same Cluster can use different secrets.
func EnsureSecretAccess(
	ctx context.Context, kube kubernetes.Interface, cluster cnpgv1.Cluster, secretNames []string,
) error {
	if err := ensureRole(ctx, kube, cluster, secretNames); err != nil {
		return err
	}
	return ensureRoleBinding(ctx, kube, cluster)
}

func ensureRole(ctx context.Context, kube kubernetes.Interface, cluster cnpgv1.Cluster, secretNames []string) error {
	roles := kube.RbacV1().Roles(cluster.Namespace)
	role, err := roles.Get(ctx, Name(cluster.Name), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = roles.Create(ctx, &rbacv1.Role{
			ObjectMeta: objectMeta(cluster),
			Rules:      secretRules(merge(nil, secretNames)),
		}, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return ensureRole(ctx, kube, cluster, secretNames)
		}
		return err
	}
	if err != nil {
		return err
	}

	var granted []string
	for _, rule := range role.Rules {
		granted = append(granted, rule.ResourceNames...)
	}
	names := merge(granted, secretNames)
	if slices.Equal(merge(nil, granted), names) && len(role.Rules) == 1 {
		return nil
	}
	role.Rules = secretRules(names)
	_, err = roles.Update(ctx, role, metav1.UpdateOptions{})
	return err
}

func ensureRoleBinding(ctx context.Context, kube kubernetes.Interface, cluster cnpgv1.Cluster) error {
	desired := &rbacv1.RoleBinding{
		ObjectMeta: objectMeta(cluster),
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: Name(cluster.Name)},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName(cluster),
			Namespace: cluster.Namespace,
		}},
	}

	bindings := kube.RbacV1().RoleBindings(cluster.Namespace)
	binding, err := bindings.Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = bindings.Create(ctx, desired, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	if err != nil {
		return err
	}
	if slices.Equal(binding.Subjects, desired.Subjects) {
		return nil
	}
	binding.Subjects = desired.Subjects
	_, err = bindings.Update(ctx, binding, metav1.UpdateOptions{})
	return err
}

func objectMeta(cluster cnpgv1.Cluster) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      Name(cluster.Name),
		Namespace: cluster.Namespace,
		Labels:    map[string]string{"app.kubernetes.io/managed-by": "cnpg-plugin-pgdump", "cnpg.io/cluster": cluster.Name},
		// deleted with the Cluster
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: cnpgv1.SchemeGroupVersion.String(),
			Kind:       cnpgv1.ClusterKind,
			Name:       cluster.Name,
			UID:        cluster.UID,
		}},
	}
}

func secretRules(names []string) []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{{
		APIGroups:     []string{""},
		Resources:     []string{"secrets"},
		Verbs:         []string{"get"},
		ResourceNames: names,
	}}
}

func merge(existing, added []string) []string {
	names := append(slices.Clone(existing), added...)
	slices.Sort(names)
	return slices.Compact(names)
}

func serviceAccountName(cluster cnpgv1.Cluster) string {
	if cluster.Spec.ServiceAccountName != "" {
		return cluster.Spec.ServiceAccountName
	}
	return cluster.Name
}
