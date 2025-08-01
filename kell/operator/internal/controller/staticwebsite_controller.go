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

	webappv1alpha1 "github.com/jesusxy/bit-by-bit/kell/operator/api/v1alpha1"
)

// StaticWebsiteReconciler reconciles a StaticWebsite object
type StaticWebsiteReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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

	foundDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: staticWebsite.Name, Namespace: staticWebsite.Namespace}, foundDeployment)

	if err != nil && errors.IsNotFound(err) {
		// define new deployment
		dep := r.deploymentForStaticWebsite(staticWebsite)
		logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)

		if err != nil {
			logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}

		// deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get deployment")
		return ctrl.Result{}, err
	}

	// ensure deployment size is the same as spec
	size := staticWebsite.Spec.Replicas
	if *foundDeployment.Spec.Replicas != size {
		foundDeployment.Spec.Replicas = &size
		err = r.Update(ctx, foundDeployment)
		if err != nil {
			logger.Error(err, "Failed to update Deployment", "Deployment.Namespace", foundDeployment.Namespace, "Deployment.Name", foundDeployment.Name)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{Requeue: true}, nil
	}

	foundService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: staticWebsite.Name, Namespace: staticWebsite.Namespace}, foundService)

	if err != nil && errors.IsNotFound(err) {
		// define a new service
		svc := r.serviceForStaticWebsite(staticWebsite)
		logger.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			logger.Error(err, "Failed to create a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	// all resources are in the desired state
	return ctrl.Result{}, nil
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
						Image: "alpine/git:latest",
						Env: []corev1.EnvVar{{
							Name:  "GIT_TERMINAL_PROMPT",
							Value: "0",
						}},
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
