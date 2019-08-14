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

const (
	// DeadMansSnitchFinalizer is used on ClusterDeployments to ensure we run a successful deprovision
	// job before cleaning up the API object.
	DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch"
	// DeadMansSnitchOperatorNamespace is the namespace where this operator will run
	DeadMansSnitchOperatorNamespace string = "deadmanssnitch-operator"
	// DeadMansSnitchAPISecretName is the secret Name where to fetch the DMS API Key
	DeadMansSnitchAPISecretName string = "deadmanssnitch-api-key"
	// DeadMansSnitchAPISecretKey is the secret where to fetch the DMS API Key
	DeadMansSnitchAPISecretKey string = "deadmanssnitch-api-key"
	// DeadMansSnitchTagKey is the secret where to fetch the DMS API Key
	DeadMansSnitchTagKey string = "hive-cluster-tag"
	// ClusterDeploymentManagedLabel is the label the clusterdeployment will have that determines
	// if the cluster is OSD (managed) or now
	ClusterDeploymentManagedLabel string = "api.openshift.com/managed"
)

var log = logf.Log.WithName("controller_deadmanssnitch")

// Add creates a new DeadMansSnitch Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	newRec, err := newReconciler(mgr)
	if err != nil {
		return err
	}

	return add(mgr, newRec)
	//return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {

	// Regular manager client is not fully initialized here, create our own for some
	// initialization API communication:
	tempClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return nil, err
	}

	// get dms key
	dmsAPIKey, err := hivecontrollerutils.LoadSecretData(tempClient, DeadMansSnitchAPISecretName,
		DeadMansSnitchOperatorNamespace, DeadMansSnitchAPISecretKey)
	if err != nil {
		return nil, err
	}

	return &ReconcileDeadMansSnitch{
		//client:    mgr.GetClient(),
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		dmsclient: dmsclient.NewClient(dmsAPIKey)}, nil
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
	dmsclient dmsclient.Client
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

	// Just return if this is not a managed cluster
	if val, ok := instance.Labels[ClusterDeploymentManagedLabel]; ok {
		if val != "true" {
			reqLogger.Info("Not a managed cluster", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, nil
		}
	} else {
		// Managed tag is not present which implies it is not a managed cluster
		reqLogger.Info("Not a managed cluster", "Namespace", request.Namespace, "Name", request.Name)
		return reconcile.Result{}, nil
	}

	// cluster isn't installed yet, just return
	if !instance.Status.Installed {
		// Cluster isn't installed yet, return
		reqLogger.Info("Cluster installation is not complete, returning...", "Namespace", request.Namespace, "Name", request.Name)
		return reconcile.Result{}, nil
	}
	reqLogger.Info("Checking to see if CD is deleted", "Namespace", request.Namespace, "Name", request.Name)
	// Check to see if the ClusterDeployment is deleted
	if instance.DeletionTimestamp != nil {
		// Delete the dms
		reqLogger.Info("Deleting the DMS from api.deadmanssnitch.com", "Namespace", request.Namespace, "Name", request.Name)
		snitchName := instance.Spec.ClusterName + "." + instance.Spec.BaseDomain
		snitches, err := r.dmsclient.FindSnitchesByName(snitchName)
		if err != nil {
			return reconcile.Result{}, err
		}
		for _, s := range snitches {
			delStatus, err := r.dmsclient.Delete(s.Token)
			if !delStatus || err != nil {
				reqLogger.Info("Failed to delete the DMS from api.deadmanssnitch.com", "Namespace", request.Namespace, "Name", request.Name)
				return reconcile.Result{}, err
			}
			reqLogger.Info("Deleted the DMS from api.deadmanssnitch.com", "Namespace", request.Namespace, "Name", request.Name)
		}

		reqLogger.Info("Deleting DMS finalizer from ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
		hivecontrollerutils.DeleteFinalizer(instance, DeadMansSnitchFinalizer)
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Error deleting Finalizer from ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}

		// Things should be cleaned up...
		return reconcile.Result{}, nil

	}

	// Add finalizer to the ClusterDeployment
	if !hivecontrollerutils.HasFinalizer(instance, DeadMansSnitchFinalizer) {
		reqLogger.Info("Adding DMS finalizer to ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
		hivecontrollerutils.AddFinalizer(instance, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Error setting Finalizer on ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}

		// Exit the reconcile loop after modifying the clusterdeployment resource
		// Adding the finalizer to clusterdeployment will requeue a reconciliation loop
		// and strange things will happen.  Just return after modifying the resource that
		// is being watched.
		return reconcile.Result{}, nil
	}

	ssName := request.Name + "-dms"
	// Check to see if the SyncSet exists
	err = r.client.Get(context.TODO(),
		types.NamespacedName{Name: ssName, Namespace: request.Namespace},
		&hivev1alpha1.SyncSet{})
	if errors.IsNotFound(err) {
		// create new DMS SyncSet
		reqLogger.Info("SyncSet not found, Creating a new SynsSet", "Namespace", request.Namespace, "Name", request.Name)

		snitchName := instance.Spec.ClusterName + "." + instance.Spec.BaseDomain
		snitches, err := r.dmsclient.FindSnitchesByName(snitchName)
		if err != nil {
			return reconcile.Result{}, err
		}

		var snitch dmsclient.Snitch
		if len(snitches) > 0 {
			snitch = snitches[0]
		} else {
			hiveClusterTag, err := hivecontrollerutils.LoadSecretData(r.client, DeadMansSnitchAPISecretName,
				DeadMansSnitchOperatorNamespace, DeadMansSnitchTagKey)
			if err != nil {
				reqLogger.Error(err, "Unable to retrieve the hive-cluster-tag from the secret", "Namespace", request.Namespace, "Name", request.Name)
				return reconcile.Result{}, err
			}
			tags := []string{hiveClusterTag}
			newSnitch := dmsclient.NewSnitch(snitchName, tags, "15_minute", "basic")
			snitch, err = r.dmsclient.Create(newSnitch)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Get the snitch again to check status
		snitches, err = r.dmsclient.FindSnitchesByName(snitchName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(snitches) > 0 {
			if snitches[0].Status == "pending" {
				reqLogger.Info("Checking in Snitch ...", "Namespace", request.Namespace, "Name", request.Name)
				// CheckIn snitch
				err = r.dmsclient.CheckIn(snitch)
				if err != nil {
					reqLogger.Error(err, "Unable to check in deadman's snitch", "Namespace", request.Namespace, "Name", request.Name, "CheckInURL", snitch.CheckInURL)
					return reconcile.Result{}, err
				}
				return reconcile.Result{}, nil
			} else {
				reqLogger.Error(err, "Unable to get Snitch by name", "Namespace", request.Namespace, "Name", request.Name)
				return reconcile.Result{}, nil
			}
		}

		newSS := newSyncSet(request.Namespace, request.Name, snitch.CheckInURL)

		// ensure the syncset gets cleaned up when the clusterdeployment is deleted
		if err := controllerutil.SetControllerReference(instance, newSS, r.scheme); err != nil {
			reqLogger.Error(err, "Error setting controller reference on syncset", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
		if err := r.client.Create(context.TODO(), newSS); err != nil {
			reqLogger.Error(err, "Error creating syncset", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}

		reqLogger.Info("Done creating a new SyncSet", "Namespace", request.Namespace, "Name", request.Name)
	} else {
		reqLogger.Info("SyncSet Already Present, nothing to do here...", "Namespace", request.Namespace, "Name", request.Name)
	}

	return reconcile.Result{}, nil

}

func newSyncSet(namespace string, clusterDeploymentName string, snitchURL string) *hivev1alpha1.SyncSet {

	newSS := &hivev1alpha1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterDeploymentName + "-dms",
			Namespace: namespace,
		},
		Spec: hivev1alpha1.SyncSetSpec{
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				{
					Name: clusterDeploymentName,
				},
			},
			SyncSetCommonSpec: hivev1alpha1.SyncSetCommonSpec{
				ResourceApplyMode: "sync",
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
								Namespace: "openshift-monitoring",
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
