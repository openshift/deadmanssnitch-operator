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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/operator-framework/operator-lib/leader"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"github.com/openshift/operator-custom-metrics/pkg/metrics"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/api/v1alpha1"
	operatorconfig "github.com/openshift/deadmanssnitch-operator/config"
	controllers "github.com/openshift/deadmanssnitch-operator/controllers/deadmanssnitchintegration"
	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"
	//+kubebuilder:scaffold:imports
)

var (
	scheme      = k8sruntime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	metricsPath = "/metrics"
	// metricsPort the port on which metrics is hosted, don't pick one that's already used
	metricsPort = "8081"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(hivev1.AddToScheme(scheme))
	utilruntime.Must(deadmanssnitchv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func printVersion() {
	setupLog.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	setupLog.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func main() {
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8080", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Print configuration info
	printVersion()
	if err := operatorconfig.SetIsFedramp(); err != nil {
		setupLog.Error(err, "failed to get fedramp value")
		os.Exit(1)
	}
	if operatorconfig.IsFedramp() {
		setupLog.Info("running in fedramp environment.")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     "0", // disable the controller-runtime metrics
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "3a94b80a.deadmanssnitch.managed.openshift.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	err = leader.Become(context.TODO(), "deadmanssnitch-operator-lock")
	if err != nil {
		setupLog.Error(err, "Failed to retry for leader lock")
		os.Exit(1)
	}

	if err = (&controllers.DeadmansSnitchIntegrationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeadmansSnitchIntegration")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Configure custom metrics
	localmetrics.Collector = localmetrics.NewMetricsCollector()
	metricsServer := metrics.NewBuilder(operatorconfig.OperatorNamespace, operatorconfig.OperatorName).
		WithPort(metricsPort).
		WithPath(metricsPath).
		WithCollector(localmetrics.Collector).
		WithRoute().
		GetConfig()

	if err := metrics.ConfigureMetrics(context.TODO(), *metricsServer); err != nil {
		setupLog.Error(err, "failed to configure custom metrics")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
