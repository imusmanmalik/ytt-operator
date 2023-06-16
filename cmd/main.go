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

package main

import (
	"context"
	"encoding/base64"
	"flag"
	"os"
	"path/filepath"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/dpeckett/ytt-operator/api/v1alpha1"
	yttoperatorv1alpha1 "github.com/dpeckett/ytt-operator/api/v1alpha1"
	"github.com/dpeckett/ytt-operator/internal/controller"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(yttoperatorv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var reconcilerName string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&reconcilerName, "reconciler-name", "",
		"The name of the reconciler configuration to use, expected to be present in the same namespace as the operator.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0a0439c1.pecke.tt",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	ctx := context.Background()
	ctrl.LoggerInto(ctx, setupLog)

	if reconcilerName != "" {
		var reconcilerConfig v1alpha1.Reconciler
		err := mgr.GetAPIReader().Get(ctx, types.NamespacedName{
			Name:      reconcilerName,
			Namespace: os.Getenv("POD_NAMESPACE"),
		}, &reconcilerConfig)
		if err != nil {
			setupLog.Error(err, "Unable to retrieve reconciler configuration")
			os.Exit(1)
		}

		scriptsDir, err := os.MkdirTemp("", "ytt-operator")
		if err != nil {
			setupLog.Error(err, "Unable to create temporary scripts directory")
			os.Exit(1)
		}
		defer os.RemoveAll(scriptsDir)

		// Write the scripts out to a temporary directory.
		for _, s := range reconcilerConfig.Spec.Scripts {
			data, err := base64.StdEncoding.DecodeString(s.Encoded)
			if err != nil {
				setupLog.Error(err, "Unable to decode script")
				os.Exit(1)
			}

			if err := os.WriteFile(filepath.Join(scriptsDir, s.Name), data, 0o644); err != nil {
				setupLog.Error(err, "Unable to write script to temporary directory")
				os.Exit(1)
			}
		}

		for _, gvk := range reconcilerConfig.Spec.For {
			// Register the reconciler for each GVK.
			if err := controller.NewYTTReconciler(mgr, gvk.GroupVersionKind(), scriptsDir).SetupWithManager(mgr); err != nil {
				setupLog.Error(err, "unable to create controller", "controller", gvk.GroupVersionKind().String())
				os.Exit(1)
			}
		}
	} else {
		// Will be used as a template for the child reconcilers.
		var parent corev1.Pod
		err := mgr.GetAPIReader().Get(ctx, types.NamespacedName{
			Name:      os.Getenv("POD_NAME"),
			Namespace: os.Getenv("POD_NAMESPACE"),
		}, &parent)
		if err != nil {
			setupLog.Error(err, "Unable to retrieve pod information")
			os.Exit(1)
		}

		// Register as an operator of operators.
		if err := controller.NewReconcilerReconciler(mgr, &parent).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create controller", "controller", "Reconciler")
			os.Exit(1)
		}
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}
