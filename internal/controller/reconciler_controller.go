/*
 * Copyright 2023 Damian Peckett <damian@pecke.tt>.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/dpeckett/ytt-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReconcilerReconciler reconciles a Reconciler object
type ReconcilerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Parent *corev1.Pod
}

//+kubebuilder:rbac:groups=ytt-operator.pecke.tt,resources=reconcilers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ytt-operator.pecke.tt,resources=reconcilers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ytt-operator.pecke.tt,resources=reconcilers/finalizers,verbs=update

// So we can manage the child reconcilers.
//+kubebuilder:rbac:groups="",resources=pods,verbs=get
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch

func NewReconcilerReconciler(mgr ctrl.Manager, parent *corev1.Pod) *ReconcilerReconciler {
	return &ReconcilerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Parent: parent,
	}
}

func (r *ReconcilerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling")

	var obj v1alpha1.Reconciler
	err := r.Get(ctx, req.NamespacedName, &obj)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Object not found")

			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get object: %w", err)
	}

	if obj.GetDeletionTimestamp() != nil {
		logger.Info("Deleting child reconciler")

		err := r.Client.Delete(ctx, &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ytt-operator-" + obj.GetName(),
				Namespace: r.Parent.GetNamespace(),
			},
		})
		if err != nil {
			if !errors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to delete child reconciler: %w", err)
			}
		}

		logger.Info("Removing finalizer")

		if err := removeFinalizer(ctx, r.Client, &obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer if it's not already present.
	if err := addFinalizer(ctx, r.Client, &obj); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
	}

	logger.Info("Reconciling child reconciler")

	child := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "ytt-operator-" + obj.GetName(), Namespace: r.Parent.GetNamespace()}}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, child, func() error {
		podSpec := r.Parent.Spec.DeepCopy()
		podSpec.ServiceAccountName = obj.Spec.ServiceAccountName

		for i, c := range podSpec.Containers {
			if c.Name == "manager" {
				podSpec.Containers[i].Args = append(podSpec.Containers[i].Args, "--reconciler-name="+obj.GetName())
				break
			}
		}

		var replicas int32 = 1
		child.Spec = appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "ytt-operator-" + obj.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "ytt-operator-" + obj.GetName(),
					},
				},
				Spec: *podSpec,
			},
		}

		return nil
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch child reconciler: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *ReconcilerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Reconciler{}).
		Complete(r)
}
