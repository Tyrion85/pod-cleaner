/*
Copyright 2022.

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

package controllers

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

type PodDeleterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

type PodDeleterReconcilerOptions struct {
	MaxConcurrentReconciles int
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodDeleterReconciler) SetupWithManager(mgr ctrl.Manager, opts PodDeleterReconcilerOptions) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}).
		Complete(r)
}

// Reconcile
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *PodDeleterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("Namespace", req.Namespace, "Name", req.NamespacedName)
	logger.Info("reconcile started")

	// Fetch the pod
	pod := corev1.Pod{}

	err := r.Client.Get(ctx, req.NamespacedName, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if pod.Status.Phase == corev1.PodFailed &&
		(pod.Status.Reason == "Terminated" && strings.Contains(pod.Status.Message, "node shutdown")) ||
		(pod.Status.Reason == "NodeShutdown" && strings.Contains(pod.Status.Message, "node is shutting down")) {
		// delete pod
		logger.Info("deleting pod")
		if err = r.deletePod(ctx, &pod); err != nil {
			logger.Error(err, "failed to delete pod", pod, pod.Name)
			return ctrl.Result{Requeue: true}, err
		}
		logger.Info("pod deleted")
	}
	return ctrl.Result{}, nil
}

func (r *PodDeleterReconciler) deletePod(ctx context.Context, pod *corev1.Pod) error {
	key := client.ObjectKeyFromObject(pod)
	latest := &corev1.Pod{}
	if err := r.Client.Get(ctx, key, latest); err != nil {
		return err
	}
	return r.Client.Delete(ctx, pod)
}
