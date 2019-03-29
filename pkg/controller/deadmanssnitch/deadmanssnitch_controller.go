package deadmanssnitch

import (
	"context"

	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	hivev1alpha1 "github.com/openshift/hive/pkg/apis/hive/v1alpha1"
	hivecontrollerutils "github.com/openshift/hive/pkg/controller/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_deadmanssnitch")

// Add creates a new DeadMansSnitch Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	// get dms key
	dmsAPIKey := "CHANGEME"
	return &ReconcileDeadMansSnitch{
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		dmsclient: dmsclient.NewClient(dmsAPIKey)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deadmanssnitch-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ClusterDeployment
	err = c.Watch(&source.Kind{Type: &hivev1alpha1.ClusterDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	/*
		// TODO(user): Modify this to be the types you create that are owned by the primary resource
		// Watch for changes to secondary resource Pods and requeue the owner DeadMansSnitch
		err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &hivev1alpha1.SyncSet{},
		})
		if err != nil {
			return err
		}
	*/

	return nil
}

var _ reconcile.Reconciler = &ReconcileDeadMansSnitch{}

// ReconcileDeadMansSnitch reconciles a DeadMansSnitch object
type ReconcileDeadMansSnitch struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	scheme    *runtime.Scheme
	dmsclient *dmsclient.Client
}

// Reconcile reads that state of the cluster for a DeadMansSnitch object and makes changes based on the state read
// and what is in the DeadMansSnitch.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDeadMansSnitch) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DeadMansSnitch")

	// Fetch the ClusterDeployment instance
	instance := &hivev1alpha1.ClusterDeployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	reqLogger.Info("Checking to see if CD is deleted", "Namespace", request.Namespace, "Name", request.Name)
	// Check to see if the ClusterDeployment is deleted
	if instance.DeletionTimestamp != nil {
		// Delete the dms
		reqLogger.Info("Deleting the DMS from api.deadmanssnicth.com", "Namespace", request.Namespace, "Name", request.Name)
		snitches, err := r.dmsclient.FindSnitchesByName(request.Name)
		if err != nil {
			return reconcile.Result{}, err
		}
		for _, s := range snitches {
			delStatus, err := r.dmsclient.Delete(s.Token)
			if !delStatus || err != nil {
				reqLogger.Info("Failed to delete the DMS from api.deadmanssnicth.com", "Namespace", request.Namespace, "Name", request.Name)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Deleted the DMS from api.deadmanssnicth.com", "Namespace", request.Namespace, "Name", request.Name)
		}

		reqLogger.Info("Deleting DMS finalizer from ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
		hivecontrollerutils.DeleteFinalizer(instance, "dms.manage.openshift.io/deadmanssnitch")
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Info("Error deleting Finalizer from ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}

		// Things should be cleaned up...
		return reconcile.Result{}, nil

	}

	// Add finalizer to the ClusterDeployment
	if !hivecontrollerutils.HasFinalizer(instance, "dms.manage.openshift.io/deadmanssnitch") {
		reqLogger.Info("Adding DMS finalizer to ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
		hivecontrollerutils.AddFinalizer(instance, "dms.manage.openshift.io/deadmanssnitch")
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Info("Error setting Finalizer on ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
	}

	ssName := request.Name + "-dms"
	// Check to see if the SyncSet exists
	err = r.client.Get(context.TODO(),
		types.NamespacedName{Name: ssName, Namespace: request.Namespace},
		&hivev1alpha1.SyncSet{})

	if errors.IsNotFound(err) {
		// create new DMS SyncSet
		reqLogger.Info("SyncSet not found!", "Namespace", request.Namespace, "Name", request.Name)
		reqLogger.Info("Creating a new SyncSet", "Namespace", request.Namespace, "Name", request.Name)

		snitches, err := r.dmsclient.FindSnitchesByName(request.Name)
		if err != nil {
			return reconcile.Result{}, err
		}

		var snitch dmsclient.Snitch
		if len(snitches) > 0 {
			snitch = snitches[0]
		} else {
			tags := []string{"production"}
			newSnitch := dmsclient.NewSnitch(request.Name, tags, "daily", "basic")
			snitch, err = r.dmsclient.Create(newSnitch)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		newSS := newSyncSet(request.Namespace, ssName, snitch.Href)

		// ensure the syncset gets cleaned up when the clusterdeployment is deleted
		if err := controllerutil.SetControllerReference(instance, newSS, r.scheme); err != nil {
			reqLogger.Info("error setting controller reference on syncset", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
		if err := r.client.Create(context.TODO(), newSS); err != nil {
			reqLogger.Info("error creating syncset", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}

		reqLogger.Info("Done creating a new SyncSet", "Namespace", request.Namespace, "Name", request.Name)
	} else {
		reqLogger.Info("Already Created, nothing to do here...", "Namespace", request.Namespace, "Name", request.Name)

	}

	return reconcile.Result{}, nil

}

func newSyncSet(namespace string, ssName string, snitchURL string) *hivev1alpha1.SyncSet {

	newSS := &hivev1alpha1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ssName,
			Namespace: namespace,
		},
		Spec: hivev1alpha1.SyncSetSpec{
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				{
					Name: ssName,
				},
			},
			SyncSetCommonSpec: hivev1alpha1.SyncSetCommonSpec{
				ResourceApplyMode: "upsert",
				Resources: []runtime.RawExtension{
					{
						Object: &corev1.Secret{
							Type: "Opaque",
							TypeMeta: metav1.TypeMeta{
								Kind:       "Secret",
								APIVersion: "v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      "dms-secret",
								Namespace: "openshift-am-config",
							},
							Data: map[string][]byte{
								"SNITCH_URL": []byte(snitchURL),
							},
						},
					},
				},
			},
		},
	}

	return newSS

}
