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

package controller_test

import (
	"context"
	"testing"

	"github.com/dpeckett/meta-operator/internal/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMetaReconciler_Reconcile(t *testing.T) {
	s := scheme.Scheme
	gvk := schema.GroupVersionKind{Group: "testGroup", Version: "testVersion", Kind: "testKind"}
	fakeClient := clientfake.NewClientBuilder().WithScheme(s).Build()

	mgr := &MockManager{}
	mgr.On("GetClient").Return(fakeClient)
	mgr.On("GetScheme").Return(s)

	r := controller.NewMetaReconciler(mgr, gvk, "testdata")

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName("test")
	obj.SetNamespace("default")

	err := fakeClient.Create(context.Background(), obj)
	assert.NoError(t, err)

	t.Run("Test object creation", func(t *testing.T) {
		_, err := r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKey{
				Name:      "test",
				Namespace: "default",
			},
		})
		assert.NoError(t, err)

		var cm corev1.ConfigMap
		err = fakeClient.Get(context.Background(), client.ObjectKey{
			Name:      "derived-configmap",
			Namespace: "default",
		}, &cm)
		assert.NoError(t, err)

		assert.Equal(t, "default", cm.Data["namespace"])
	})
}

type MockManager struct {
	mock.Mock
	ctrl.Manager
}

func (mgr *MockManager) GetClient() client.Client {
	args := mgr.Called()
	return args.Get(0).(client.Client)
}

func (mgr *MockManager) GetScheme() *runtime.Scheme {
	args := mgr.Called()
	return args.Get(0).(*runtime.Scheme)
}
