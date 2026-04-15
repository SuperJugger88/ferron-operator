/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "SuperJugger88/ferron-operator/api/v1alpha1"
)

const (
	ferronSystemNS      = "ferron-system"
	ferronProxyName     = "ferron-operator"
	ferronConfigMapName = "ferron-operator-config"
	ferronServiceName   = "ferron-operator"
	defaultImage        = "ferronserver/ferron:latest"
	reloadPort          = 8189
)

var log = logf.Log.WithName("ferronproxy-controller")

type FerronProxyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferronproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferronproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferronproxies/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferroncertificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferroncertificates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferronproxyselectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.ferron.sh,resources=ferronproxyselectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;create
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;create;update;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;create;update;delete
// +kubebuilder:rbac:groups="",resources=deployments,verbs=get;create;update;delete
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update;delete

func (r *FerronProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Starting reconciliation", "req", req.Name)

	_, err := r.getWatchNamespaces(ctx)
	if err != nil {
		logger.Error(err, "Failed to get namespaces to watch")
		return ctrl.Result{}, err
	}

	if err := r.ensureNamespace(ctx); err != nil {
		logger.Error(err, "Failed to ensure ferron-system namespace")
		return ctrl.Result{}, err
	}

	if err := r.ensureDefaultProxySelector(ctx); err != nil {
		logger.Error(err, "Failed to ensure default FerronProxySelector")
		return ctrl.Result{}, err
	}

	selector, err := r.getDefaultProxySelector(ctx)
	if err != nil {
		logger.Error(err, "Failed to get FerronProxySelector")
		return ctrl.Result{}, err
	}

	image := selector.Spec.Image
	if image == "" {
		image = defaultImage
	}

	replicas := selector.Spec.Replicas
	if replicas == nil {
		replicas = new(int32)
		*replicas = 1
	}

	serviceType := selector.Spec.ServiceType
	if serviceType == "" {
		serviceType = "ClusterIP"
	}

	ferronProxyList := &networkingv1alpha1.FerronProxyList{}
	if err := r.List(ctx, ferronProxyList, &client.ListOptions{Namespace: ""}); err != nil {
		logger.Error(err, "Failed to list FerronProxy")
		return ctrl.Result{}, err
	}

	certList := &networkingv1alpha1.FerronCertificateList{}
	if err := r.List(ctx, certList, &client.ListOptions{Namespace: ""}); err != nil {
		logger.Error(err, "Failed to list FerronCertificate")
		return ctrl.Result{}, err
	}

	certMap := make(map[string]*networkingv1alpha1.FerronCertificate)
	for i := range certList.Items {
		cert := &certList.Items[i]
		certMap[cert.Spec.Domain] = cert
	}

	filteredProxyes := r.filterProxyesBySelector(ferronProxyList.Items, "ferron")

	if err := r.ensureDeployment(ctx, image, *replicas); err != nil {
		logger.Error(err, "Failed to ensure ferron-proxy deployment")
		return ctrl.Result{}, err
	}

	if err := r.ensureService(ctx, serviceType); err != nil {
		logger.Error(err, "Failed to ensure ferron-proxy service")
		return ctrl.Result{}, err
	}

	kdlConfig := r.generateKDLConfig(filteredProxyes, certMap)
	if err := r.ensureConfigMap(ctx, kdlConfig); err != nil {
		logger.Error(err, "Failed to ensure configmap")
		return ctrl.Result{}, err
	}

	if err := r.triggerReload(ctx); err != nil {
		logger.Error(err, "Failed to trigger ferron reload")
	}

	for i := range ferronProxyList.Items {
		proxy := &ferronProxyList.Items[i]
		if err := r.updateProxyStatus(ctx, proxy); err != nil {
			logger.Error(err, "Failed to update proxy status", "name", proxy.Name)
		}
	}

	logger.Info("Reconciliation completed")
	return ctrl.Result{}, nil
}

func (r *FerronProxyReconciler) getWatchNamespaces(ctx context.Context) ([]string, error) {
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: ferronSystemNS}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	namespacesAnnotation := ns.Annotations["ferron.sh/namespaces"]
	if namespacesAnnotation == "" {
		return nil, nil
	}

	nsList := strings.Split(namespacesAnnotation, ",")
	for i := range nsList {
		nsList[i] = strings.TrimSpace(nsList[i])
	}
	return nsList, nil
}

func (r *FerronProxyReconciler) ensureNamespace(ctx context.Context) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ferronSystemNS}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, ns, func() error {
		return nil
	})
	return err
}

func (r *FerronProxyReconciler) ensureDefaultProxySelector(ctx context.Context) error {
	existing := &networkingv1alpha1.FerronProxySelector{}
	err := r.Get(ctx, types.NamespacedName{Name: "ferron"}, existing)
	if err == nil {
		if existing.Spec.ControllerName != "ferron.ferron.sh/controller" {
			existing.Spec.ControllerName = "ferron.ferron.sh/controller"
			return r.Client.Update(ctx, existing)
		}
		return nil
	}
	if !apierrors.IsNotFound(err) {
		log.V(1).Info("Could not get FerronProxySelector, skipping creation", "error", err)
		return nil
	}

	selector := &networkingv1alpha1.FerronProxySelector{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ferron",
		},
		Spec: networkingv1alpha1.FerronProxySelectorSpec{
			ControllerName: "ferron.ferron.sh/controller",
			Image:          defaultImage,
			Replicas:       new(int32),
			ServiceType:    "LoadBalancer",
		},
	}
	*selector.Spec.Replicas = 1
	err = r.Client.Create(ctx, selector)
	if err != nil {
		log.V(1).Info("Could not create FerronProxySelector, skipping", "error", err)
		return nil
	}
	return nil
}

func (r *FerronProxyReconciler) getProxySelectorName(ctx context.Context) (string, error) {
	selectorList := &networkingv1alpha1.FerronProxySelectorList{}
	if err := r.List(ctx, selectorList, &client.ListOptions{Namespace: ""}); err != nil {
		return "", err
	}

	for _, selector := range selectorList.Items {
		return selector.Name, nil
	}

	return "ferron", nil
}

func (r *FerronProxyReconciler) getDefaultProxySelector(ctx context.Context) (*networkingv1alpha1.FerronProxySelector, error) {
	selector := &networkingv1alpha1.FerronProxySelector{}
	err := r.Get(ctx, types.NamespacedName{Name: "ferron"}, selector)
	if err != nil {
		return nil, err
	}
	return selector, nil
}

func (r *FerronProxyReconciler) filterProxyesBySelector(proxies []networkingv1alpha1.FerronProxy, selectorName string) []networkingv1alpha1.FerronProxy {
	var filtered []networkingv1alpha1.FerronProxy
	for _, proxy := range proxies {
		if proxy.Spec.ProxySelector == "" {
			filtered = append(filtered, proxy)
		} else if proxy.Spec.ProxySelector == selectorName {
			filtered = append(filtered, proxy)
		}
	}
	return filtered
}

func (r *FerronProxyReconciler) ensureDeployment(ctx context.Context, image string, replicas int32) error {
	deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: ferronProxyName, Namespace: ferronSystemNS}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		deploy.Spec.Replicas = &replicas
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": ferronProxyName},
		}
		deploy.Spec.Template.ObjectMeta.Labels = map[string]string{"app": ferronProxyName}
		deploy.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  "ferron",
				Image: image,
				Ports: []corev1.ContainerPort{
					{Name: "http", ContainerPort: 80},
					{Name: "https", ContainerPort: 443},
					{Name: "admin", ContainerPort: int32(reloadPort)},
				},
				VolumeMounts: []corev1.VolumeMount{
					{Name: "config", MountPath: "/etc/ferron", ReadOnly: true},
				},
			},
		}
		deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: ferronConfigMapName},
					},
				},
			},
		}
		return nil
	})
	return err
}

func (r *FerronProxyReconciler) ensureService(ctx context.Context, serviceType string) error {
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: ferronServiceName, Namespace: ferronSystemNS}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Type = corev1.ServiceType(serviceType)
		svc.Spec.Selector = map[string]string{"app": ferronProxyName}
		svc.Spec.Ports = []corev1.ServicePort{
			{Name: "http", Port: 80, TargetPort: intstr.FromInt(80)},
			{Name: "https", Port: 443, TargetPort: intstr.FromInt(443)},
		}
		return nil
	})
	return err
}

func (r *FerronProxyReconciler) ensureConfigMap(ctx context.Context, content string) error {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: ferronConfigMapName, Namespace: ferronSystemNS}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		cm.Data = map[string]string{"ferron.kdl": content}
		return nil
	})
	return err
}

func (r *FerronProxyReconciler) generateKDLConfig(proxies []networkingv1alpha1.FerronProxy, certs map[string]*networkingv1alpha1.FerronCertificate) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("server {"))
	lines = append(lines, fmt.Sprintf("    port 80"))
	lines = append(lines, fmt.Sprintf("    port 443"))
	lines = append(lines, fmt.Sprintf("}"))

	for _, proxy := range proxies {
		for _, route := range proxy.Spec.Config.Routes {
			host := route.Host
			if host == "" {
				host = "*"
			}

			for _, handle := range route.Handle {
				location := handle.Location
				if location == "" {
					location = "/"
				}

				scheme := handle.Proxy.Service.Scheme
				if scheme == "" {
					scheme = "http"
				}

				endpoint := handle.Proxy.Service.Endpoint
				if endpoint == "" {
					endpoint = "/"
				}

				serviceName := handle.Proxy.Service.Name
				port := handle.Proxy.Service.Port.Number

				lines = append(lines, fmt.Sprintf("%q:80 {", host))
				lines = append(lines, fmt.Sprintf("    location %q {", location))

				backendURL := fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d%s",
					scheme, serviceName, proxy.Namespace, port, endpoint)
				lines = append(lines, fmt.Sprintf("        proxy %q", backendURL))
				lines = append(lines, fmt.Sprintf("    }"))
				lines = append(lines, fmt.Sprintf("}"))
			}
		}

		if proxy.Spec.TLS != nil && proxy.Spec.TLS.Certificate != "" {
			for _, route := range proxy.Spec.Config.Routes {
				host := route.Host
				if host == "" {
					host = "*"
				}

				cert, ok := certs[host]
				if !ok {
					continue
				}

				secretName := cert.Spec.SecretName
				if secretName == "" {
					secretName = fmt.Sprintf("ferron-tls-%s", cert.Name)
				}

				lines = append(lines, fmt.Sprintf("%q:443 {", host))
				lines = append(lines, fmt.Sprintf("    tls {"))
				lines = append(lines, fmt.Sprintf("        secret %q", secretName))

				if cert.Spec.ACMEServer != "" {
					lines = append(lines, fmt.Sprintf("        acme {"))
					lines = append(lines, fmt.Sprintf("            email %q", cert.Spec.Email))
					lines = append(lines, fmt.Sprintf("            server %q", cert.Spec.ACMEServer))
					lines = append(lines, fmt.Sprintf("        }"))
				} else {
					lines = append(lines, fmt.Sprintf("        acme {"))
					lines = append(lines, fmt.Sprintf("            email %q", cert.Spec.Email))
					lines = append(lines, fmt.Sprintf("        }"))
				}

				lines = append(lines, fmt.Sprintf("    }"))
				lines = append(lines, fmt.Sprintf("}"))
			}
		}
	}

	return strings.Join(lines, "\n")
}

func (r *FerronProxyReconciler) triggerReload(ctx context.Context) error {
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/reload", ferronProxyName, ferronSystemNS, reloadPort)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (r *FerronProxyReconciler) updateProxyStatus(ctx context.Context, proxy *networkingv1alpha1.FerronProxy) error {
	proxy.Status.Conditions = []metav1.Condition{
		{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "Reconciled",
			Message:            "FerronProxy has been reconciled",
		},
	}

	return r.Status().Update(ctx, proxy)
}

func (r *FerronProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.FerronProxy{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("ferronproxy").
		Complete(r)
}
