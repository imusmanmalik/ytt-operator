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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dpeckett/ytt-operator/api/v1alpha1"
	"github.com/dpeckett/ytt-operator/internal/controller"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestReconcilerReconciler(t *testing.T) {
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

	crd, err := loadCRD("../../config/crd/bases/ytt-operator.pecke.tt_reconcilers.yaml")
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
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Port:   0,
	})
	require.NoError(t, err)

	parent := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "manager",
					Image: "k8s.gcr.io/pause:3.9",
				},
			},
		},
	}

	r := controller.NewReconcilerReconciler(mgr, parent)
	err = r.SetupWithManager(mgr)
	require.NoError(t, err)

	go func() {
		if err := mgr.Start(ctx); err != nil {
			t.Log(err)
		}
	}()

	t.Run("Test child reconciler creation", func(t *testing.T) {
		obj := &v1alpha1.Reconciler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
			},
			Spec: v1alpha1.ReconcilerSpec{
				For: []metav1.TypeMeta{
					{
						Kind:       "Deployment",
						APIVersion: "apps/v1",
					},
				},
				Scripts: []string{"test"},
			},
		}

		err := r.Client.Create(ctx, obj)
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

		// Wait for the deployment to be created.
		var d *appsv1.Deployment
		err = wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
			d, err = clientset.AppsV1().Deployments("default").Get(ctx, "ytt-operator-test", metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return true, nil
		})
		require.NoError(t, err)

		// Check that the deployment has the correct arguments.
		assert.Equal(t, "--reconciler-name=test", d.Spec.Template.Spec.Containers[0].Args[0], "Reconciler name should be set")
	})
}

func loadCRD(path string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdYAML, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load CRD: %w", err)
	}

	// construct a new scheme and add the types we need
	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to construct scheme: %w", err)
	}

	crd, _, err := serializer.NewCodecFactory(scheme).UniversalDeserializer().Decode(crdYAML, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode CRD: %w", err)
	}

	return crd.(*apiextensionsv1.CustomResourceDefinition), nil
}
