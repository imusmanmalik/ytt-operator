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
	"time"

	"github.com/dpeckett/ytt-operator/api/v1alpha1"
	"github.com/dpeckett/ytt-operator/internal/controller"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestYTTReconciler(t *testing.T) {
	logConfig := zap.NewDevelopmentConfig()
	logConfig.DisableStacktrace = true

	logger, err := logConfig.Build()
	require.NoError(t, err)
	ctrl.SetLogger(zapr.NewLogger(logger))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = ctrl.LoggerInto(ctx, ctrl.Log)

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	require.NoError(t, err)

	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	crdClientset, err := apiextensionsclientset.NewForConfig(config)
	require.NoError(t, err)

	crd, err := loadCRD("../../config/crd/bases/ytt-operator.pecke.tt_testresources.yaml")
	require.NoError(t, err)

	existing, err := crdClientset.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		t.Fatal(err)
	}

	if err == nil {
		crd.ResourceVersion = existing.ResourceVersion
		_, err = crdClientset.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, crd, metav1.UpdateOptions{})
	} else {
		_, err = crdClientset.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	}
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Port:   0,
	})
	require.NoError(t, err)

	gvk := schema.GroupVersionKind{Group: v1alpha1.GroupVersion.Group, Version: v1alpha1.GroupVersion.Version, Kind: "TestResource"}

	r := controller.NewYTTReconciler(mgr, gvk, []string{"testdata"})
	err = r.SetupWithManager(mgr)
	require.NoError(t, err)

	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Log(err)
		}
	}()

	t.Run("Test object creation", func(t *testing.T) {
		obj := &v1alpha1.TestResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: v1alpha1.TestResourceSpec{
				Foo: "bar",
			},
		}

		err = r.Client.Create(ctx, obj)
		require.NoError(t, err)

		defer func() {
			t.Log("Cleaning up test object")

			if err := r.Client.Delete(ctx, obj); err != nil {
				t.Log(err)
			}

			err := wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				err := r.Client.Get(ctx, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, obj)
				if err != nil {
					return true, nil
				}
				return false, nil
			})
			if err != nil {
				t.Log(err)
			}
		}()

		var cm *corev1.ConfigMap
		err = wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			cm, err = clientset.CoreV1().ConfigMaps("default").Get(ctx,
				"derived-configmap-test", metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return true, nil
		})
		require.NoError(t, err)

		assert.Equal(t, "default", cm.Data["namespace"])
	})
}
