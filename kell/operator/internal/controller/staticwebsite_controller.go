/*
Copyright 2025.

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
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	webappv1alpha1 "github.com/jesusxy/bit-by-bit/kell/operator/api/v1alpha1"
)

// StaticWebsiteReconciler reconciles a StaticWebsite object
type StaticWebsiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// rbac = Role Based Access Controls

//+kubebuilder:rbac:groups=webapp.com,resources=staticwebsites,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webapp.com,resources=staticwebsites/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webapp.com,resources=staticwebsites/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the StaticWebsite object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *StaticWebsiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	staticWebsite := &webappv1alpha1.StaticWebsite{}
	err := r.Get(ctx, req.NamespacedName, staticWebsite)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("StaticWebsite resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get StaticWebsite")
		return ctrl.Result{}, err
	}

	// --- Deployment Reconciliation ---
	foundDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: staticWebsite.Name, Namespace: staticWebsite.Namespace}, foundDeployment)

	if err != nil && errors.IsNotFound(err) {
		// create new dployment
		desiredDeployment := r.deploymentForStaticWebsite(staticWebsite)
		logger.Info("Creating New Deployment", "Deployment.Namespace", desiredDeployment.Namespace, "Deployment.Name", desiredDeployment.Name)
		err = r.Create(ctx, desiredDeployment)
		if err != nil {
			logger.Error(err, "Failed to create new Deployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get deployment")
		return ctrl.Result{}, err
	}

	desiredDeployment := r.deploymentForStaticWebsite(staticWebsite)
	if r.deploymentNeedsUpdate(foundDeployment, desiredDeployment, logger) {
		logger.Info("Deployment spec is out of date, updating...")

		r.updateDeploymentSpec(foundDeployment, desiredDeployment)

		err = r.Update(ctx, foundDeployment)
		if err != nil {
			logger.Error(err, "Failed to update deployment")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// --- Service Reconciliation ---\
	foundService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: staticWebsite.Name, Namespace: staticWebsite.Namespace}, foundService)

	if err != nil && errors.IsNotFound(err) {
		// define a new service
		desiredService := r.serviceForStaticWebsite(staticWebsite)
		logger.Info("Creating a new Service", "Service.Namespace", desiredService.Namespace, "Service.Name", desiredService.Name)
		err = r.Create(ctx, desiredService)
		if err != nil {
			logger.Error(err, "Failed to create a new Service", "Service.Namespace", desiredService.Namespace, "Service.Name", desiredService.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// a real world operator would also compare the desiredService.Spec with the foundService.Spec and update if needed

	// all resources are in the desired state
	return ctrl.Result{}, nil
}

func (r *StaticWebsiteReconciler) deploymentNeedsUpdate(found, desired *appsv1.Deployment, logger logr.Logger) bool {
	if found.Spec.Replicas == nil || *found.Spec.Replicas != *desired.Spec.Replicas {
		logger.Info("Replica count differs", "found", found.Spec.Replicas, "desired", *desired.Spec.Replicas)
		return true
	}

	foundInitContainers := r.normalizeContainers(found.Spec.Template.Spec.InitContainers)
	desiredInitContainers := r.normalizeContainers(desired.Spec.Template.Spec.InitContainers)

	if !reflect.DeepEqual(foundInitContainers, desiredInitContainers) {
		logger.Info("Init containers differ")
		logger.Info("Found init containers", "containers", foundInitContainers)
		logger.Info("Desired init containers", "containers", desiredInitContainers)
		return true
	}

	foundContainers := r.normalizeContainers(found.Spec.Template.Spec.Containers)
	desiredContainers := r.normalizeContainers(desired.Spec.Template.Spec.Containers)

	if !reflect.DeepEqual(foundContainers, desiredContainers) {
		logger.Info("Main containers differ")
		logger.Info("Found containers", "containers", foundContainers)
		logger.Info("Desired containers", "containers", desiredContainers)
		return true
	}

	if !reflect.DeepEqual(found.Spec.Template.Spec.Volumes, desired.Spec.Template.Spec.Volumes) {
		logger.Info("Volumes differ")
		logger.Info("Found volumes", "volumes", found.Spec.Template.Spec.Volumes)
		logger.Info("Desired volumes", "volumes", desired.Spec.Template.Spec.Volumes)
		return true
	}

	return false
}

func (r *StaticWebsiteReconciler) normalizeContainers(containers []corev1.Container) []corev1.Container {
	normalized := make([]corev1.Container, len(containers))
	copy(normalized, containers)

	for i := range normalized {
		// clear fields that Kubernetes auto-populates
		normalized[i].TerminationMessagePath = ""
		normalized[i].TerminationMessagePolicy = ""
		normalized[i].ImagePullPolicy = ""

		if normalized[i].Resources.Limits == nil && normalized[i].Resources.Requests == nil {
			normalized[i].Resources = corev1.ResourceRequirements{}
		}

		if normalized[i].SecurityContext != nil {
			if normalized[i].SecurityContext.RunAsNonRoot == nil &&
				normalized[i].SecurityContext.ReadOnlyRootFilesystem == nil &&
				normalized[i].SecurityContext.AllowPrivilegeEscalation == nil &&
				normalized[i].SecurityContext.RunAsUser == nil &&
				normalized[i].SecurityContext.RunAsGroup == nil {
				normalized[i].SecurityContext = nil
			}
		}
	}

	return normalized
}

func (r *StaticWebsiteReconciler) updateDeploymentSpec(found, desired *appsv1.Deployment) {
	found.Spec.Replicas = desired.Spec.Replicas
	found.Spec.Template.Spec.InitContainers = desired.Spec.Template.Spec.InitContainers
	found.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
	found.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes

	if !reflect.DeepEqual(found.Spec.Template.ObjectMeta.Labels, desired.Spec.Template.ObjectMeta.Labels) {
		found.Spec.Template.ObjectMeta.Labels = desired.Spec.Template.ObjectMeta.Labels
	}
}

func (r *StaticWebsiteReconciler) deploymentForStaticWebsite(sw *webappv1alpha1.StaticWebsite) *appsv1.Deployment {
	labels := map[string]string{"app": sw.Name}
	replicas := sw.Spec.Replicas

	webContentVolume := corev1.Volume{
		Name: "web-content",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sw.Name,
			Namespace: sw.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{webContentVolume},
					InitContainers: []corev1.Container{{
						Name:  "git-content",
						Image: "bitnami/git:latest",
						Env: []corev1.EnvVar{{
							Name:  "GIT_TERMINAL_PROMPT",
							Value: "0",
						}},
						Command: []string{"git"},
						Args: []string{
							"clone",
							"--single-branch",
							"--",
							sw.Spec.GitRepo,
							"/web-content",
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "web-content",
							MountPath: "/web-content",
						}},
					}},
					Containers: []corev1.Container{{
						Image: "nginx:latest",
						Name:  "web-server",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "http",
							Protocol:      corev1.ProtocolTCP,
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "web-content",
							MountPath: "/usr/share/nginx/html",
							ReadOnly:  true,
						}},
					}},
				},
			},
		},
	}

	ctrl.SetControllerReference(sw, dep, r.Scheme)
	return dep
}

func (r *StaticWebsiteReconciler) serviceForStaticWebsite(sw *webappv1alpha1.StaticWebsite) *corev1.Service {
	labels := map[string]string{"app": sw.Name}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sw.Name,
			Namespace: sw.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Protocol:   "TCP",
				Port:       80,
				TargetPort: intstr.FromInt(80),
			}},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
	ctrl.SetControllerReference(sw, svc, r.Scheme)
	return svc
}

// SetupWithManager sets up the controller with the Manager.
func (r *StaticWebsiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1alpha1.StaticWebsite{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
