package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/chakradharkondapalli/topas/api/v1alpha1"
)

// TestRunReconciler reconciles a TestRun object
type TestRunReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps.example.com,resources=testruns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.example.com,resources=testruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *TestRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var testRun appv1alpha1.TestRun
	if err := r.Get(ctx, req.NamespacedName, &testRun); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 1. Handle Pending State
	if testRun.Status.State == "" || testRun.Status.State == "Pending" {
		log.Info("Reconciling Pending TestRun", "name", testRun.Name)

		// Check Concurrency
		var activePods corev1.PodList
		if err := r.List(ctx, &activePods, client.MatchingLabels{"runner-type": "topas"}); err != nil {
			return ctrl.Result{}, err
		}

		runningCount := 0
		for _, p := range activePods.Items {
			if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodPending {
				runningCount++
			}
		}

		const MaxConcurrency = 5
		if runningCount >= MaxConcurrency {
			log.Info("Concurrency limit reached", "active", runningCount, "limit", MaxConcurrency)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		// Create ConfigMap for inline script
		if testRun.Spec.Script != "" {
			cm := r.defineScriptConfigMap(&testRun)
			if err := ctrl.SetControllerReference(&testRun, cm, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.Create(ctx, cm); err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "Failed to create script ConfigMap")
				return ctrl.Result{}, err
			}
		}

		// Create Runner Pod
		pod := r.defineRunnerPod(&testRun)
		if err := ctrl.SetControllerReference(&testRun, pod, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference on runner pod")
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, pod); err != nil {
			log.Error(err, "Failed to create runner pod")
			return ctrl.Result{}, err
		}

		// Update Status to Running
		testRun.Status.State = "Running"
		testRun.Status.RunnerPod = pod.Name
		now := metav1.Now()
		testRun.Status.StartTime = &now
		if err := r.Status().Update(ctx, &testRun); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// 2. Handle Running State
	if testRun.Status.State == "Running" {
		var pod corev1.Pod
		podName := types.NamespacedName{Name: testRun.Status.RunnerPod, Namespace: testRun.Namespace}
		if err := r.Get(ctx, podName, &pod); err != nil {
			if errors.IsNotFound(err) {
				testRun.Status.State = "Error"
				testRun.Status.Result = "Runner Pod not found"
				r.Status().Update(ctx, &testRun)
			}
			return ctrl.Result{}, err
		}

		// Check Pod Status
		if pod.Status.Phase == corev1.PodSucceeded {
			testRun.Status.State = "Passed"
			testRun.Status.Result = "Success"
			now := metav1.Now()
			testRun.Status.CompletionTime = &now
			r.Status().Update(ctx, &testRun)
		} else if pod.Status.Phase == corev1.PodFailed {
			testRun.Status.State = "Failed"
			testRun.Status.Result = "Runner Pod Failed"
			now := metav1.Now()
			testRun.Status.CompletionTime = &now
			r.Status().Update(ctx, &testRun)
		} else {
			// Pod still running — requeue to check again
			log.Info("Runner pod still running, will recheck", "pod", pod.Name)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
	}

	return ctrl.Result{}, nil
}

// defineScriptConfigMap creates a ConfigMap containing the inline Lua script.
func (r *TestRunReconciler) defineScriptConfigMap(run *appv1alpha1.TestRun) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.Name + "-script",
			Namespace: run.Namespace,
			Labels:    map[string]string{"testrun": run.Name, "runner-type": "topas"},
		},
		Data: map[string]string{
			"test.lua": run.Spec.Script,
		},
	}
}

func (r *TestRunReconciler) defineRunnerPod(run *appv1alpha1.TestRun) *corev1.Pod {
	scriptPath := "/scripts/test.lua"

	// Parse timeout → activeDeadlineSeconds
	var activeDeadline *int64
	if run.Spec.Timeout != "" {
		if d, err := time.ParseDuration(run.Spec.Timeout); err == nil {
			secs := int64(d.Seconds())
			activeDeadline = &secs
		}
	}

	// Determine app name and namespace for runner args
	appName := run.Spec.AppName
	if appName == "" {
		appName = "unknown"
	}

	// Base Pod
	runnerImage := os.Getenv("RUNNER_IMAGE")
	if runnerImage == "" {
		runnerImage = "localhost/topas-runner:latest"
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      run.Name + "-runner",
			Namespace: run.Namespace,
			Labels:    map[string]string{"testrun": run.Name, "runner-type": "topas"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: activeDeadline,
			Containers: []corev1.Container{{
				Name:            "runner",
				Image:           runnerImage,
				ImagePullPolicy: corev1.PullNever,
				Args: []string{
					"--script", scriptPath,
					"--app", appName,
					"--namespace", run.Namespace,
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "scripts",
					MountPath: "/scripts",
				}},
			}},
		},
	}

	// Mount script source
	if run.Spec.Script != "" {
		// Use ConfigMap volume for inline scripts (robust, handles all characters)
		pod.Spec.Volumes = []corev1.Volume{{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: run.Name + "-script",
					},
				},
			},
		}}
	} else if run.Spec.Git != nil {
		// Use emptyDir + git init container for git-sourced scripts
		pod.Spec.Volumes = []corev1.Volume{{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}}

		cmd := fmt.Sprintf("git clone %s /scripts", run.Spec.Git.URL)
		if run.Spec.Git.Revision != "" {
			cmd += fmt.Sprintf(" && cd /scripts && git checkout %s", run.Spec.Git.Revision)
		}

		pod.Spec.InitContainers = []corev1.Container{{
			Name:    "init-git",
			Image:   "alpine/git",
			Command: []string{"sh", "-c", cmd},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "scripts",
				MountPath: "/scripts",
			}},
		}}
	} else {
		// Fallback: empty volume
		pod.Spec.Volumes = []corev1.Volume{{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}}
	}

	return pod
}

func (r *TestRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.TestRun{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
