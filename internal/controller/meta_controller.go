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
	"io"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type MetaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	gvk    schema.GroupVersionKind
	dir    string
}

func NewMetaReconciler(mgr ctrl.Manager, gvk schema.GroupVersionKind, dir string) *MetaReconciler {
	return &MetaReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		gvk:    gvk,
		dir:    dir,
	}
}

func (r *MetaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling")

	var obj unstructured.Unstructured
	obj.SetGroupVersionKind(r.gvk)

	err := r.Get(ctx, req.NamespacedName, &obj)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get object: %w", err)
	}

	// Because we use owner references, we don't need to do anything here.
	if obj.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, nil
	}

	objYAML, err := yaml.Marshal(obj.Object)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to marshal object: %w", err)
	}

	logger.Info("invoking ytt")

	cmd := exec.Command("ytt", "-f", r.dir, "-f", "-")
	cmd.Stdin = strings.NewReader("#@data/values\n---\n" + string(objYAML))
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(err, "ytt failed", "output", string(out))

		return ctrl.Result{}, fmt.Errorf("ytt failed: %w", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(out))
	for {
		var child unstructured.Unstructured
		if err := dec.Decode(&child.Object); err != nil {
			if err == io.EOF {
				break
			}

			return ctrl.Result{}, fmt.Errorf("failed to unmarshal object: %w", err)
		}

		if child.GetObjectKind().GroupVersionKind().Empty() {
			return ctrl.Result{}, fmt.Errorf("object has unknown type: %v", child)
		}

		logger.Info("creating or updating object", "object", child)

		_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &child, func() error {
			if err := ctrl.SetControllerReference(&obj, &child, r.Scheme); err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create or update object: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MetaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	var obj unstructured.Unstructured
	obj.SetGroupVersionKind(r.gvk)

	return ctrl.NewControllerManagedBy(mgr).
		For(&obj).
		Complete(r)
}
