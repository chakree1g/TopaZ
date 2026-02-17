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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1alpha1 "github.com/chakradharkondapalli/topas/api/v1alpha1"
)

// AppReconciler reconciles a App object
type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.example.com,resources=apps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.example.com,resources=apps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.example.com,resources=apps/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch the App CR
	var app appsv1alpha1.App
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling App", "name", app.Name, "services", len(app.Spec.Services), "databases", len(app.Spec.Databases))

	managedResources := make(map[string]bool)

	// 2. Reconcile databases first (services may depend on them)
	for _, db := range app.Spec.Databases {
		name := fmt.Sprintf("%s-%s", app.Name, db.Name)
		managedResources[name] = true

		if err := r.reconcileDatabase(ctx, &app, db, name); err != nil {
			log.Error(err, "Failed to reconcile Database", "name", name)
			return ctrl.Result{}, err
		}
	}

	// 3. Reconcile each service → Deployment + Service
	for _, svc := range app.Spec.Services {
		name := fmt.Sprintf("%s-%s", app.Name, svc.Name)
		managedResources[name] = true

		if err := r.reconcileDeployment(ctx, &app, svc, name); err != nil {
			log.Error(err, "Failed to reconcile Deployment", "name", name)
			return ctrl.Result{}, err
		}
		if err := r.reconcileService(ctx, &app, svc, name); err != nil {
			log.Error(err, "Failed to reconcile Service", "name", name)
			return ctrl.Result{}, err
		}
	}

	// 4. Clean up orphaned Deployments (services removed from spec)
	var depList appsv1.DeploymentList
	if err := r.List(ctx, &depList, client.InNamespace(app.Namespace), client.MatchingLabels{
		"app.kubernetes.io/managed-by": "topas",
		"app.kubernetes.io/part-of":    app.Name,
	}); err != nil {
		return ctrl.Result{}, err
	}
	for i := range depList.Items {
		dep := &depList.Items[i]
		if !managedResources[dep.Name] {
			log.Info("Deleting orphaned Deployment", "name", dep.Name)
			if err := r.Delete(ctx, dep); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			orphanSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: dep.Name, Namespace: app.Namespace}}
			if err := r.Delete(ctx, orphanSvc); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		}
	}

	// 5. Update status
	app.Status.EndpointCount = int32(len(app.Spec.Services) + len(app.Spec.Databases))
	app.Status.Health = "Healthy"
	if err := r.Status().Update(ctx, &app); err != nil {
		log.Error(err, "Failed to update App status")
		return ctrl.Result{}, err
	}

	log.Info("App reconciled successfully", "name", app.Name)
	return ctrl.Result{}, nil
}

// reconcileDatabase creates a Deployment + Service for a database, and a schema init Job if InitSQL is set.
func (r *AppReconciler) reconcileDatabase(ctx context.Context, app *appsv1alpha1.App, db appsv1alpha1.DatabaseSpec, name string) error {
	log := logf.FromContext(ctx)

	labels := map[string]string{
		"app":                          db.Name,
		"app.kubernetes.io/name":       db.Name,
		"app.kubernetes.io/managed-by": "topas",
		"app.kubernetes.io/part-of":    app.Name,
		"app.kubernetes.io/component":  "database",
	}

	// Build env vars from credentials
	envVars := []corev1.EnvVar{}
	if user, ok := db.Credentials["user"]; ok {
		envVars = append(envVars, corev1.EnvVar{Name: "POSTGRES_USER", Value: user})
	}
	if pass, ok := db.Credentials["password"]; ok {
		envVars = append(envVars, corev1.EnvVar{Name: "POSTGRES_PASSWORD", Value: pass})
	}
	if dbname, ok := db.Credentials["dbname"]; ok {
		envVars = append(envVars, corev1.EnvVar{Name: "POSTGRES_DB", Value: dbname})
	}

	replicas := int32(1)
	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            db.Name,
						Image:           db.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env:             envVars,
						Ports: []corev1.ContainerPort{{
							ContainerPort: db.Port,
						}},
					}},
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: app.Namespace}, &existing)
	if errors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		existing.Spec = desired.Spec
		existing.Labels = desired.Labels
		if err := r.Update(ctx, &existing); err != nil {
			return err
		}
	}

	// Reconcile Service
	svcDesired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Port:       db.Port,
				TargetPort: intstr.FromInt32(db.Port),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}

	if err := ctrl.SetControllerReference(app, svcDesired, r.Scheme); err != nil {
		return err
	}

	var existingSvc corev1.Service
	err = r.Get(ctx, types.NamespacedName{Name: name, Namespace: app.Namespace}, &existingSvc)
	if errors.IsNotFound(err) {
		if err := r.Create(ctx, svcDesired); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		existingSvc.Spec.Ports = svcDesired.Spec.Ports
		existingSvc.Spec.Selector = svcDesired.Spec.Selector
		existingSvc.Labels = svcDesired.Labels
		if err := r.Update(ctx, &existingSvc); err != nil {
			return err
		}
	}

	// Schema init Job (only create if InitSQL is set and Job doesn't exist yet)
	if db.InitSQL != "" {
		jobName := name + "-init"
		var existingJob batchv1.Job
		err := r.Get(ctx, types.NamespacedName{Name: jobName, Namespace: app.Namespace}, &existingJob)
		if errors.IsNotFound(err) {
			log.Info("Creating schema init Job", "name", jobName)

			user := db.Credentials["user"]
			pass := db.Credentials["password"]
			dbname := db.Credentials["dbname"]
			host := name // Service name = DB hostname

			// psql command to run init SQL
			psqlCmd := fmt.Sprintf(
				`until pg_isready -h %s -p %d -U %s; do echo "waiting for db..."; sleep 2; done; PGPASSWORD=%s psql -h %s -p %d -U %s -d %s -c '%s'`,
				host, db.Port, user,
				pass, host, db.Port, user, dbname,
				db.InitSQL,
			)

			backoffLimit := int32(3)
			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      jobName,
					Namespace: app.Namespace,
					Labels:    labels,
				},
				Spec: batchv1.JobSpec{
					BackoffLimit: &backoffLimit,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{{
								Name:    "init-schema",
								Image:   "postgres:16-alpine",
								Command: []string{"sh", "-c", psqlCmd},
							}},
						},
					},
				},
			}

			if err := ctrl.SetControllerReference(app, job, r.Scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, job); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (r *AppReconciler) reconcileDeployment(ctx context.Context, app *appsv1alpha1.App, svc appsv1alpha1.ServiceSpec, name string) error {
	replicas := int32(1)
	if svc.Replicas != nil {
		replicas = *svc.Replicas
	}

	labels := map[string]string{
		"app":                          svc.Name,
		"app.kubernetes.io/name":       svc.Name,
		"app.kubernetes.io/managed-by": "topas",
		"app.kubernetes.io/part-of":    app.Name,
	}

	// Build env vars from ServiceSpec
	envVars := []corev1.EnvVar{}
	for k, v := range svc.EnvVars {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            svc.Name,
						Image:           svc.Image,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env:             envVars,
						Ports: []corev1.ContainerPort{{
							ContainerPort: svc.Port,
						}},
					}},
				},
			},
		},
	}
	// Add gRPC port to container if specified
	if svc.GrpcPort != nil {
		desired.Spec.Template.Spec.Containers[0].Ports = append(desired.Spec.Template.Spec.Containers[0].Ports, corev1.ContainerPort{
			ContainerPort: *svc.GrpcPort,
			Name:          "grpc",
		})
	}

	// Set owner reference for garbage collection
	if err := ctrl.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	// Check if Deployment exists
	var existing appsv1.Deployment
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: app.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update existing Deployment
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	return r.Update(ctx, &existing)
}

func (r *AppReconciler) reconcileService(ctx context.Context, app *appsv1alpha1.App, svc appsv1alpha1.ServiceSpec, name string) error {
	labels := map[string]string{
		"app":                          svc.Name,
		"app.kubernetes.io/name":       svc.Name,
		"app.kubernetes.io/managed-by": "topas",
		"app.kubernetes.io/part-of":    app.Name,
	}

	ports := []corev1.ServicePort{{
		Name:       "http",
		Port:       svc.Port,
		TargetPort: intstr.FromInt32(svc.Port),
		Protocol:   corev1.ProtocolTCP,
	}}

	if svc.GrpcPort != nil {
		ports = append(ports, corev1.ServicePort{
			Name:       "grpc",
			Port:       *svc.GrpcPort,
			TargetPort: intstr.FromInt32(*svc.GrpcPort),
			Protocol:   corev1.ProtocolTCP,
		})
	}

	desired := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: app.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    ports,
		},
	}

	// Set owner reference for garbage collection
	if err := ctrl.SetControllerReference(app, desired, r.Scheme); err != nil {
		return err
	}

	// Check if Service exists
	var existing corev1.Service
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: app.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Update existing: preserve ClusterIP
	existing.Spec.Ports = desired.Spec.Ports
	existing.Spec.Selector = desired.Spec.Selector
	existing.Labels = desired.Labels
	return r.Update(ctx, &existing)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1alpha1.App{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("app").
		Complete(r)
}
