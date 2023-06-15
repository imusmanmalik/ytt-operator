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

	"github.com/dpeckett/ytt-operator/internal/controller"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestYTTReconciler_Reconcile(t *testing.T) {
	logConfig := zap.NewDevelopmentConfig()
	logConfig.DisableStacktrace = true

	logger, err := logConfig.Build()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = ctrl.LoggerInto(ctx, zapr.NewLogger(logger))
	defer cancel()

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err)

	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	crdClientset, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testresources.ytt-operator.example.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "ytt-operator.example.com",
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "testresources",
				Singular: "testresource",
				Kind:     "TestResource",
			},
		},
	}

	existing, err := crdClientset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err == nil {
		crd.ResourceVersion = existing.ResourceVersion
		_, err = crdClientset.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
	} else {
		_, err = crdClientset.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	}
	require.NoError(t, err)

	gvk := schema.GroupVersionKind{Group: "ytt-operator.example.com", Version: "v1", Kind: "TestResource"}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(gvk.GroupVersion(), &TestResource{})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
	})
	require.NoError(t, err)

	r := controller.NewYTTReconciler(mgr, gvk, []string{"testdata"})
	err = r.SetupWithManager(mgr)
	require.NoError(t, err)

	t.Run("Test object creation", func(t *testing.T) {
		client, err := dynamic.NewForConfig(config)
		require.NoError(t, err)

		gvr := schema.GroupVersionResource{Group: "ytt-operator.example.com", Version: "v1", Resource: "testresources"}
		testResource := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "ytt-operator.example.com/v1",
				"kind":       "TestResource",
				"metadata": map[string]interface{}{
					"name":      "test",
					"namespace": "default",
				},
			},
		}

		_, err = client.Resource(gvr).Namespace("default").Create(ctx, testResource, metav1.CreateOptions{})
		require.NoError(t, err)

		defer func() {
			ch := make(chan struct{})

			go func() {
				defer close(ch)

			}()

			t.Log("Cleaning up test object")

			err = client.Resource(gvr).Namespace("default").Delete(ctx, "test", metav1.DeleteOptions{})
			if err != nil {
				t.Log(err)
			}

			_, err = r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test",
					Namespace: "default",
				},
			})
			if err != nil {
				t.Log(err)
			}
		}()

		_, err = r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      "test",
				Namespace: "default",
			},
		})
		require.NoError(t, err)

		cm, err := clientset.CoreV1().ConfigMaps("default").Get(ctx,
			"derived-configmap-test", metav1.GetOptions{})
		require.NoError(t, err)

		assert.Equal(t, "default", cm.Data["namespace"])
	})
}

type TestResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (t *TestResource) DeepCopyObject() runtime.Object {
	return &TestResource{}
}
