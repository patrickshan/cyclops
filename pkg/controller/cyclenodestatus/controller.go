package cyclenodestatus

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	v1 "github.com/atlassian-labs/cyclops/pkg/apis/atlassian/v1"
	"github.com/atlassian-labs/cyclops/pkg/cloudprovider"
	cyclecontroller "github.com/atlassian-labs/cyclops/pkg/controller"
	"github.com/atlassian-labs/cyclops/pkg/controller/cyclenodestatus/transitioner"
)

const (
	controllerName       = "cyclenodestatus.controller"
	eventName            = "cyclops"
	reconcileConcurrency = 16
)

var log = logf.Log.WithName(controllerName)

// Reconciler reconciles CycleNodeStatuses. It implements reconcile.Reconciler
type Reconciler struct {
	mgr           manager.Manager
	cloudProvider cloudprovider.CloudProvider
	rawClient     kubernetes.Interface
	namespace     string
}

// NewReconciler returns a new Reconciler for CycleNodeStatuses, which implements reconcile.Reconciler
// The Reconciler is registered as a controller and initialised as part of the creation.
func NewReconciler(
	mgr manager.Manager,
	cloudProvider cloudprovider.CloudProvider,
	namespace string,
) (reconcile.Reconciler, error) {
	rawClient := kubernetes.NewForConfigOrDie(mgr.GetConfig())

	// Create the reconciler
	reconciler := &Reconciler{
		mgr:           mgr,
		cloudProvider: cloudProvider,
		rawClient:     rawClient,
	}

	// Create the new controller using the reconciler. This registers it with the main event loop.
	cnsController, err := controller.New(
		controllerName,
		mgr,
		controller.Options{
			Reconciler:              reconciler,
			MaxConcurrentReconciles: reconcileConcurrency,
		})
	if err != nil {
		log.Error(err, "Unable to create cycleNodeStatus controller")
		return nil, err
	}

	// Initialise the controller's required watches
	err = cnsController.Watch(
		&source.Kind{Type: &v1.CycleNodeStatus{}},
		&handler.EnqueueRequestForObject{},
		cyclecontroller.NewNamespacePredicate(namespace),
	)
	if err != nil {
		return nil, err
	}

	// Setup an indexer for pod spec.nodeName
	err = mgr.GetFieldIndexer().IndexField(&coreV1.Pod{}, "spec.nodeName", func(object runtime.Object) []string {
		p, ok := object.(*coreV1.Pod)
		if !ok {
			return []string{}
		}
		return []string{p.Spec.NodeName}
	})
	if err != nil {
		return nil, err
	}
	return reconciler, nil
}

// Reconcile reconciles the incoming request, usually a cycleNodeStatus
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logger := log.WithValues("name", request.Name, "namespace", request.Namespace, "controller", controllerName)

	// Fetch the CycleNodeStatus from the API server
	cycleNodeStatus := &v1.CycleNodeStatus{}
	err := r.mgr.GetClient().Get(context.TODO(), request.NamespacedName, cycleNodeStatus)
	if err != nil {
		// Object not found, must have been deleted
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		logger.Error(err, "Failed to get cycleNodeStatus")
		return reconcile.Result{}, err
	}

	logger = log.WithValues("name", request.Name, "namespace", request.Namespace, "phase", cycleNodeStatus.Status.Phase)
	rm := cyclecontroller.NewResourceManager(
		r.mgr.GetClient(),
		r.rawClient,
		r.mgr.GetEventRecorderFor(eventName),
		logger,
		r.cloudProvider)
	result, err := transitioner.NewCycleNodeStatusTransitioner(cycleNodeStatus, rm).Run()
	return result, err
}