package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	operatorconfig "github.com/openshift/deadmanssnitch-operator/config"
	"github.com/openshift/deadmanssnitch-operator/pkg/apis"
	"github.com/openshift/deadmanssnitch-operator/pkg/controller"
	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"
	"github.com/openshift/operator-custom-metrics/pkg/metrics"

	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"

	//	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"

	routev1 "github.com/openshift/api/route/v1"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/pflag"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

// Change below variables to serve metrics on different host or port.
var (
	metricsPath = "/metrics"
	metricsPort = "8080"
)
var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}

func main() {
	// Add the zap logger flag set to the CLI. The flag set must
	// be added before calling pflag.Parse().
	pflag.CommandLine.AddFlagSet(zap.FlagSet())

	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	pflag.Parse()

	// Use a zap logr.Logger implementation. If none of the zap
	// flags are configured (or if the zap flag set is not being
	// used), this defaults to a production zap logger.
	//
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(zap.Logger())

	printVersion()

	/*
		namespace, err := k8sutil.GetWatchNamespace()
		if err != nil {
			log.Error(err, "Failed to get watch namespace")
			os.Exit(1)
		}
	*/

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Become the leader before proceeding
	err = leader.Become(ctx, "deadmanssnitch-operator-lock")
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace: "",
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup hive scheme
	if err := hivev1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "error registering route objects")
		os.Exit(1)
	}

	localmetrics.Collector = localmetrics.NewMetricsCollector()
	metricsServer := metrics.NewBuilder().WithPort(metricsPort).WithPath(metricsPath).
		WithCollector(localmetrics.Collector).
		WithRoute().
		WithServiceName(operatorconfig.OperatorName).
		GetConfig()

	// Configure metrics if it errors log the error but continue
	if err := metrics.ConfigureMetrics(context.TODO(), *metricsServer); err != nil {
		log.Error(err, "Failed to configure Metrics")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}
