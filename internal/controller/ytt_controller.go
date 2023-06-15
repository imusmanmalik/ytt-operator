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
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/dpeckett/ytt-operator/internal/util"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const finalizer = "ytt-operator.damian.pecke.tt"

type YTTReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	gvk     schema.GroupVersionKind
	scripts []string
}

func NewYTTReconciler(mgr ctrl.Manager, gvk schema.GroupVersionKind, scripts []string) *YTTReconciler {
	return &YTTReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		gvk:     gvk,
		scripts: scripts,
	}
}

func (r *YTTReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciling")

	var obj unstructured.Unstructured
	obj.SetGroupVersionKind(r.gvk)

	err := r.Get(ctx, req.NamespacedName, &obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get object: %w", err)
	}

	if obj.GetDeletionTimestamp() != nil {
		logger.Info("Deleting object resources using kapp")

		cmd := exec.CommandContext(ctx, "kapp", "delete", "-y", "-a", obj.GetName())
		cmd.Stdout = util.NewKappLogInterceptor(logger, false)
		cmd.Stderr = util.NewKappLogInterceptor(logger, true)

		if err := cmd.Run(); err != nil {
			logger.Error(err, "Kapp delete failed")

			return ctrl.Result{}, fmt.Errorf("kapp delete failed: %w", err)
		}

		logger.Info("Removing finalizer")

		obj.SetFinalizers(remove(obj.GetFinalizers(), finalizer))

		if err := r.Update(ctx, &obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update object: %w", err)
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer if it's not already present.
	if !contains(obj.GetFinalizers(), finalizer) {
		logger.Info("Adding finalizer")

		obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))

		if err := r.Update(ctx, &obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update object: %w", err)
		}
	}

	objYAML, err := yaml.Marshal(obj.Object)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal object: %w", err)
	}

	var args []string
	for _, script := range r.scripts {
		args = append(args, "-f", script)
	}
	args = append(args, "-f", "-")

	logger.Info("Invoking ytt")

	cmd := exec.CommandContext(ctx, "ytt", args...)
	cmd.Stdin = strings.NewReader("#@data/values\n---\n" + string(objYAML))
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(err, "Ytt failed", "output", string(out))

		return ctrl.Result{}, fmt.Errorf("ytt failed: %w", err)
	}

	logger.Info("Deploying manifests using kapp")

	cmd = exec.CommandContext(ctx, "kapp", "deploy", "-y", "-a", obj.GetName(), "-f", "-")
	cmd.Stdin = bytes.NewReader(out)
	cmd.Stdout = util.NewKappLogInterceptor(logger, false)
	cmd.Stderr = util.NewKappLogInterceptor(logger, true)

	if err := cmd.Run(); err != nil {
		logger.Error(err, "Kapp deploy failed", "output", string(out))

		return ctrl.Result{}, fmt.Errorf("kapp deploy failed: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *YTTReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var obj unstructured.Unstructured
	obj.SetGroupVersionKind(r.gvk)

	return ctrl.NewControllerManagedBy(mgr).
		For(&obj).
		Complete(r)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}

	return false
}

func remove(s []string, e string) []string {
	var result []string

	for _, a := range s {
		if a != e {
			result = append(result, a)
		}
	}

	return result
}
