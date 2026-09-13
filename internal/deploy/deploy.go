package deploy

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	releaseName = "costra-collector"
	saName      = "costra-collector"
)

type Config struct {
	Namespace   string
	ClusterID   string
	ClusterName string
	AgentToken  string
	IngestURL   string
	AgentImage  string
}

func Install(ctx context.Context, clientset kubernetes.Interface, cfg Config) error {
	ns := cfg.Namespace
	if ns == "" {
		ns = "costra-agent"
	}

	if err := ensureNamespace(ctx, clientset, ns); err != nil {
		return err
	}
	if err := ensureServiceAccount(ctx, clientset, ns); err != nil {
		return err
	}
	if err := ensureRBAC(ctx, clientset, ns); err != nil {
		return err
	}
	if err := ensureSecret(ctx, clientset, ns, cfg.ClusterID, cfg.AgentToken); err != nil {
		return err
	}
	_ = clientset.AppsV1().Deployments(ns).Delete(ctx, releaseName, metav1.DeleteOptions{})
	if err := ensureDaemonSet(ctx, clientset, ns, cfg); err != nil {
		return err
	}
	return nil
}

func ensureNamespace(ctx context.Context, c kubernetes.Interface, ns string) error {
	_, err := c.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	_, err = c.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns, Labels: map[string]string{
			"app.kubernetes.io/name": "costra-collector",
		}},
	}, metav1.CreateOptions{})
	return err
}

func ensureServiceAccount(ctx context.Context, c kubernetes.Interface, ns string) error {
	_, err := c.CoreV1().ServiceAccounts(ns).Get(ctx, saName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	_, err = c.CoreV1().ServiceAccounts(ns).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: ns},
	}, metav1.CreateOptions{})
	return err
}

func ensureRBAC(ctx context.Context, c kubernetes.Interface, ns string) error {
	roleName := releaseName
	_, err := c.RbacV1().ClusterRoles().Get(ctx, roleName, metav1.GetOptions{})
	if err != nil {
		_, err = c.RbacV1().ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: roleName},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"nodes", "pods", "namespaces", "services", "persistentvolumeclaims", "resourcequotas"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"apps"}, Resources: []string{"deployments", "replicasets", "statefulsets", "daemonsets"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"autoscaling"}, Resources: []string{"horizontalpodautoscalers"}, Verbs: []string{"get", "list", "watch"}},
				{APIGroups: []string{"metrics.k8s.io"}, Resources: []string{"nodes", "pods"}, Verbs: []string{"get", "list"}},
				{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: []string{"get", "create", "update"}},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	_, err = c.RbacV1().ClusterRoleBindings().Get(ctx, roleName, metav1.GetOptions{})
	if err != nil {
		_, err = c.RbacV1().ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: roleName},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     roleName,
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      saName,
				Namespace: ns,
			}},
		}, metav1.CreateOptions{})
	}
	return err
}

func ensureSecret(ctx context.Context, c kubernetes.Interface, ns, clusterID, token string) error {
	name := "costra-collector-credentials"
	existing, err := c.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		_, err = c.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			StringData: map[string]string{
				"api-token":  token,
				"cluster-id": clusterID,
			},
		}, metav1.CreateOptions{})
		return err
	}
	existing.StringData = map[string]string{
		"api-token":  token,
		"cluster-id": clusterID,
	}
	_, err = c.CoreV1().Secrets(ns).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

func ensureDaemonSet(ctx context.Context, c kubernetes.Interface, ns string, cfg Config) error {
	name := releaseName
	image := cfg.AgentImage
	if image == "" {
		image = "costra/collector-agent:0.1.0"
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: saName,
		Tolerations: []corev1.Toleration{{
			Operator: corev1.TolerationOpExists,
		}},
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(65532),
			RunAsGroup:   int64Ptr(65532),
			FSGroup:      int64Ptr(65532),
		},
		Containers: []corev1.Container{{
			Name:            "collector-agent",
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports:           []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
			Env: []corev1.EnvVar{
				{Name: "CLUSTER_ID", ValueFrom: envFromSecret("costra-collector-credentials", "cluster-id")},
				{Name: "CLUSTER_NAME", Value: cfg.ClusterName},
				{Name: "INGEST_URL", Value: cfg.IngestURL},
				{Name: "API_TOKEN", ValueFrom: envFromSecret("costra-collector-credentials", "api-token")},
				{Name: "NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
				}},
				{Name: "AGENT_NAMESPACE", Value: ns},
				{Name: "INTERVAL_MINUTES", Value: "5"},
				{Name: "HTTP_PORT", Value: "8080"},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString("http")},
				},
				InitialDelaySeconds: 10,
				PeriodSeconds:       30,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString("http")},
				},
				InitialDelaySeconds: 5,
				PeriodSeconds:       15,
			},
		}},
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec:       podSpec,
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
		},
	}

	existing, err := c.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		_, err = c.AppsV1().DaemonSets(ns).Create(ctx, ds, metav1.CreateOptions{})
		return err
	}
	ds.ResourceVersion = existing.ResourceVersion
	_, err = c.AppsV1().DaemonSets(ns).Update(ctx, ds, metav1.UpdateOptions{})
	return err
}

func envFromSecret(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: name},
			Key:                  key,
		},
	}
}

func int64Ptr(v int64) *int64 { return &v }
func boolPtr(v bool) *bool    { return &v }
