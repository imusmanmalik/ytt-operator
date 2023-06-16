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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const finalizer = "ytt-operator.damian.pecke.tt"

func addFinalizer(ctx context.Context, c client.Client, obj client.Object) error {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		// finalizer already present, nothing to do
		return nil
	}

	clone := obj.DeepCopyObject().(client.Object)
	controllerutil.AddFinalizer(clone, finalizer)

	return c.Patch(ctx, clone, client.MergeFrom(obj))
}

func removeFinalizer(ctx context.Context, c client.Client, obj client.Object) error {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		// finalizer already absent, nothing to do
		return nil
	}

	clone := obj.DeepCopyObject().(client.Object)
	controllerutil.RemoveFinalizer(clone, finalizer)

	return c.Patch(ctx, clone, client.MergeFrom(obj))
}
