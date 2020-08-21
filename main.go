// Copyright 2018 RedHat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"flag"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	"github.com/openshift/deadmanssnitch-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"os"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openshift/deadmanssnitch-operator/controllers"
	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	"github.com/openshift/operator-custom-metrics/pkg/metrics"
	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	// +kubebuilder:scaffold:imports
)

// Change below variables to serve metrics on different host or port.
var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	metricsPath = "/metrics"
	metricsPort = "8080"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func main() {
	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), manager.Options{
		Scheme:    scheme,
		Namespace: "",
		// disable the controller-runtime metrics
		MetricsBindAddress:      "0",
		LeaderElection:          true,
		LeaderElectionID:        "deadmanssnitch-operator-lock",
		LeaderElectionNamespace: controllers.OperatorNamespace,
	})
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	setupLog.Info("Registering Components.")
	// Setup hive scheme
	if err := hivev1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "error registering route objects")
		os.Exit(1)
	}
	localmetrics.Collector = localmetrics.NewMetricsCollector()
	metricsServer := metrics.NewBuilder().WithPort(metricsPort).WithPath(metricsPath).
		WithCollector(localmetrics.Collector).
		WithRoute().
		WithServiceName(controllers.OperatorName).
		GetConfig()

	// Configure metrics if it errors log the error but continue
	if err := metrics.ConfigureMetrics(context.TODO(), *metricsServer); err != nil {
		setupLog.Error(err, "failed to configure Metrics")
		os.Exit(1)
	}

	// get dms key
	dmsAPIKey, err := utils.LoadSecretData(mgr.GetClient(), controllers.DeadMansSnitchAPISecretName,
		controllers.DeadMansSnitchOperatorNamespace, controllers.DeadMansSnitchAPISecretKey)
	if err != nil {
		setupLog.Error(err, "failed to get Dead Mans's Snitch credentials")
		os.Exit(1)
	}

	snitchClient := dmsclient.NewClient(dmsAPIKey, localmetrics.Collector)
	if err = (&controllers.DeadMansSnitchReconciler{
		DmsClient: snitchClient,
		Client:    utils.NewClientWithMetricsOrDie(setupLog, mgr, "deadmanssnitch"),
		Log:       ctrl.Log.WithName("controllers").WithName("DeadMansSnitch"),
		Scheme:    mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DeadMansSnitch")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
