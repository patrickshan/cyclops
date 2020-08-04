package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/atlassian-labs/cyclops/pkg/apis"
	"github.com/atlassian-labs/cyclops/pkg/cloudprovider/builder"
	"github.com/atlassian-labs/cyclops/pkg/controller/cyclenoderequest"
	cnrTransitioner "github.com/atlassian-labs/cyclops/pkg/controller/cyclenoderequest/transitioner"
	"github.com/atlassian-labs/cyclops/pkg/controller/cyclenodestatus"
	"github.com/atlassian-labs/cyclops/pkg/metrics"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"gopkg.in/alecthomas/kingpin.v2"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var (
	version = "undefined" // replaced by ldflags at buildtime
	app     = kingpin.New("cyclops", "Kubernetes operator to rotate a group of nodes").DefaultEnvars().Version(version)

	debug = app.Flag("debug", "Run with debug logging").Short('d').Bool()

	cloudProviderName = app.Flag("cloud-provider", "Which cloud provider to use, options: [aws]").Default("aws").String()
	addr              = app.Flag("address", "Address to listen on for /metrics").Default(":8080").String()
	namespace         = app.Flag("namespace", "Namespace to watch for cycle request objects").Default("kube-system").String()

	deleteCNR        = app.Flag("delete-cnr", "Whether or not to automatically delete CNRs").Default("false").Bool()
	deleteCNRExpiry  = app.Flag("delete-cnr-expiry", "Delete the CNR this long after it was created and is successful").Default("168h").Duration()
	deleteCNRRequeue = app.Flag("delete-cnr-requeue", "How often to check if a CNR can be deleted").Default("24h").Duration()
)

var log = logf.Log.WithName("cmd")

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))
	logf.SetLogger(logf.ZapLogger(*debug))
	printVersion()

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Unable to get config")
		os.Exit(1)
	}

	ctx := context.TODO()

	// Become the leader before proceeding
	err = leader.Become(ctx, "cyclops-lock")
	if err != nil {
		log.Error(err, "Unable to become leader")
		os.Exit(1)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		// Watch all namespaces, so we can see pods in namespaces other than the current
		Namespace:          "",
		MetricsBindAddress: *addr,
	})
	if err != nil {
		log.Error(err, "Unable to create a new manager")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "Unable to setup scheme")
		os.Exit(1)
	}

	// Register the custom metrics
	metrics.Register(mgr.GetClient(), log, *namespace)

	// Setup the cloud provider
	cloudProvider, err := builder.BuildCloudProvider(*cloudProviderName)
	if err != nil {
		log.Error(err, "Unable to build cloud provider")
		os.Exit(1)
	}

	// Configure the CNR transitioner options
	cnrOptions := cnrTransitioner.Options{
		DeleteCNR:        *deleteCNR,
		DeleteCNRExpiry:  *deleteCNRExpiry,
		DeleteCNRRequeue: *deleteCNRRequeue,
	}

	// Set up and register the controllers that will share resources between them
	_, err = cyclenoderequest.NewReconciler(mgr, cloudProvider, *namespace, cnrOptions)
	if err != nil {
		log.Error(err, "Unable to add cycleNodeRequest controller")
		os.Exit(1)
	}
	_, err = cyclenodestatus.NewReconciler(mgr, cloudProvider, *namespace)
	if err != nil {
		log.Error(err, "Unable to add cycleNodeStatus controller")
		os.Exit(1)
	}

	log.Info("Starting the Cmd.")

	// Start the Cmd
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "Manager exited non-zero")
		os.Exit(1)
	}
}

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("operator-sdk Version: %v", sdkVersion.Version))
	log.Info(fmt.Sprintf("cyclops Version: %v", version))
}