package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"
	"github.com/openshift/deadmanssnitch-operator/pkg/utils"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var log = logf.Log.WithName("controller_deadmanssnitch")

// DeadMansSnitchReconciler reconciles a DeadMansSnitch object
type DeadMansSnitchReconciler struct {
	// This Client, initialized using mgr.Client() above, is a split Client
	// that reads objects from the cache and writes to the apiserver
	Client    client.Client
	Scheme    *runtime.Scheme
	DmsClient dmsclient.Client
	Log       logr.Logger
}

func (r *DeadMansSnitchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&hivev1.ClusterDeployment{}).Complete(r)
}

// Reconcile reads that state of the cluster for a DeadMansSnitch object and makes changes based on the state read
// and what is in the DeadMansSnitch.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *DeadMansSnitchReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DeadMansSnitch")

	start := time.Now()
	defer func() {
		reconcileDuration := time.Since(start).Seconds()
		reqLogger.WithValues("Duration", reconcileDuration).Info("Reconcile complete.")
		localmetrics.Collector.ObserveReconcile(reconcileDuration)
	}()

	// Fetch the ClusterDeployment instance
	processCD, instance, err := utils.CheckClusterDeployment(request, r.Client, reqLogger, SyncSetPostfix, ClusterDeploymentManagedLabel, ClusterDeploymentNoalertsLabel)

	if err != nil {
		// something went wrong, requeue
		return reconcile.Result{}, err
	}

	if !processCD {
		return reconcile.Result{}, deleteDMS(r, request, instance, reqLogger)
	}

	// Add finalizer to the ClusterDeployment
	if !utils.HasFinalizer(instance, DeadMansSnitchFinalizer) {
		reqLogger.Info("Adding DMS finalizer to ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
		utils.AddFinalizer(instance, DeadMansSnitchFinalizer)
		err := r.Client.Update(context.TODO(), instance)
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

	ssName := request.Name + SyncSetPostfix
	refSecretName := request.Name + RefSecretPostfix

	// Check to see if the SyncSet exists
	err = r.Client.Get(context.TODO(),
		types.NamespacedName{Name: ssName, Namespace: request.Namespace},
		&hivev1.SyncSet{})

	if errors.IsNotFound(err) {
		// create new DMS SyncSet
		reqLogger.Info("SyncSet not found, Creating a new SynsSet", "Namespace", request.Namespace, "Name", request.Name)

		newSS := newSyncSet(request.Namespace, refSecretName, request.Name)

		// ensure the syncset gets cleaned up when the clusterdeployment is deleted
		if err := controllerutil.SetControllerReference(instance, newSS, r.Scheme); err != nil {
			reqLogger.Error(err, "Error setting controller reference on syncset", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
		if err := r.Client.Create(context.TODO(), newSS); err != nil {
			reqLogger.Error(err, "Error creating syncset", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Done creating a new SyncSet", "Namespace", request.Namespace, "Name", request.Name)

	} else {
		reqLogger.Info("SyncSet Already Present, nothing to do here...", "Namespace", request.Namespace, "Name", request.Name)
		// return directly if the syscset already existed
		return reconcile.Result{}, nil
	}

	// Check if the secret exists
	err = r.Client.Get(context.TODO(),
		types.NamespacedName{Name: refSecretName, Namespace: request.Namespace},
		&corev1.Secret{})

	if errors.IsNotFound(err) {
		// create new secret which will be referenced by SyncSet
		reqLogger.Info("Secret not found, Creating a new Secret", "Namespace", request.Namespace, "Name", request.Name)

		snitchName := instance.Spec.ClusterName + "." + instance.Spec.BaseDomain
		snitches, err := r.DmsClient.FindSnitchesByName(snitchName)
		if err != nil {
			return reconcile.Result{}, err
		}

		var snitch dmsclient.Snitch
		if len(snitches) > 0 {
			snitch = snitches[0]
		} else {
			hiveClusterTag, err := utils.LoadSecretData(r.Client, DeadMansSnitchAPISecretName,
				DeadMansSnitchOperatorNamespace, DeadMansSnitchTagKey)
			if err != nil {
				reqLogger.Error(err, "Unable to retrieve the hive-cluster-tag from the secret", "Namespace", request.Namespace, "Name", request.Name)
				return reconcile.Result{}, err
			}
			tags := []string{hiveClusterTag}
			newSnitch := dmsclient.NewSnitch(snitchName, tags, "15_minute", "basic")
			snitch, err = r.DmsClient.Create(newSnitch)
			if err != nil {
				return reconcile.Result{}, err
			}
		}

		// Get the snitch again to check status
		ReSnitches, err := r.DmsClient.FindSnitchesByName(snitchName)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(ReSnitches) > 0 {
			if ReSnitches[0].Status == "pending" {
				reqLogger.Info("Checking in Snitch ...", "Namespace", request.Namespace, "Name", request.Name)
				// CheckIn snitch
				err = r.DmsClient.CheckIn(snitch)
				if err != nil {
					reqLogger.Error(err, "Unable to check in deadman's snitch", "Namespace", request.Namespace, "Name", request.Name, "CheckInURL", snitch.CheckInURL)
					return reconcile.Result{}, err
				}
			}
		} else {
			reqLogger.Error(err, "Unable to get Snitch by name", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}

		newRefSecret := newRefSecret(request.Namespace, refSecretName, ReSnitches[0].CheckInURL)

		// set the owner reference about the secret for gabage collection
		if err := controllerutil.SetControllerReference(instance, newRefSecret, r.Scheme); err != nil {
			reqLogger.Error(err, "Error setting controller refernce on secret", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
		// Create the secret
		if err := r.Client.Create(context.TODO(), newRefSecret); err != nil {
			reqLogger.Error(err, "Failed to create secret", "Namespace", request.Namespace, "Name", request.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Secret created in the Namespace", "Namespace", request.Namespace, "Name", request.Name)
	} else {
		reqLogger.Info("Secret already present, do not need to create...", "Namespace", request.Namespace, "Name", request.Name)
	}

	return reconcile.Result{}, nil
}

func newSyncSet(namespace string, refSecretName string, clusterDeploymentName string) *hivev1.SyncSet {

	newSS := &hivev1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterDeploymentName + SyncSetPostfix,
			Namespace: namespace,
		},
		Spec: hivev1.SyncSetSpec{
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				{
					Name: clusterDeploymentName,
				},
			},
			SyncSetCommonSpec: hivev1.SyncSetCommonSpec{
				ResourceApplyMode: hivev1.SyncResourceApplyMode,
				// Use SecretReference here which comsume the secret in the cluster namespace,
				// instead of embed the secret in the SyncSet directly
				Secrets: []hivev1.SecretMapping{
					{
						SourceRef: hivev1.SecretReference{
							Name:      refSecretName,
							Namespace: namespace,
						},
						TargetRef: hivev1.SecretReference{
							Name:      "dms-secret",
							Namespace: "openshift-monitoring",
						},
					},
				},
			},
		},
	}

	return newSS

}

// Create a new secret in the cluster namespace which contains the snitch_url as data
// and will be referenced by SyncSet
func newRefSecret(namespace string, name string, snitchURL string) *corev1.Secret {

	newRefSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			KeySnitchURL: []byte(snitchURL),
		},
	}

	return newRefSecret

}

func deleteDMS(r *DeadMansSnitchReconciler, request reconcile.Request, instance *hivev1.ClusterDeployment, reqLogger logr.Logger) error {
	// only do something if the finalizer is set
	if !utils.HasFinalizer(instance, DeadMansSnitchFinalizer) {
		return nil
	}

	// Delete the dms
	reqLogger.Info("Deleting the DMS from api.deadmanssnitch.com", "Namespace", request.Namespace, "Name", request.Name)
	snitchName := instance.Spec.ClusterName + "." + instance.Spec.BaseDomain
	snitches, err := r.DmsClient.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	for _, s := range snitches {
		delStatus, err := r.DmsClient.Delete(s.Token)
		if !delStatus || err != nil {
			reqLogger.Info("Failed to delete the DMS from api.deadmanssnitch.com", "Namespace", request.Namespace, "Name", request.Name)
			return err
		}
		reqLogger.Info("Deleted the DMS from api.deadmanssnitch.com", "Namespace", request.Namespace, "Name", request.Name)
	}

	// Delete the SyncSet
	reqLogger.Info("Deleting DMS SyncSet", "Namespace", request.Namespace, "Name", request.Name)
	err = utils.DeleteSyncSet(request.Name+SyncSetPostfix, request.Namespace, r.Client, reqLogger)
	if err != nil {
		reqLogger.Error(err, "Error deleting SyncSet", "Namespace", request.Namespace, "Name", request.Name+SyncSetPostfix)
		return err
	}

	// Delete the referenced secret
	reqLogger.Info("Deleting DMS referenced secret", "Namespace", request.Namespace, "Name", request.Name)
	err = utils.DeleteRefSecret(request.Name+RefSecretPostfix, request.Namespace, r.Client, reqLogger)
	if err != nil {
		reqLogger.Error(err, "Error deleting secret", "Namespace", request.Namespace, "Name", request.Name)
		return err
	}

	reqLogger.Info("Deleting DMS finalizer from ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
	utils.DeleteFinalizer(instance, DeadMansSnitchFinalizer)
	err = r.Client.Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Error deleting Finalizer from ClusterDeployment", "Namespace", request.Namespace, "Name", request.Name)
		return err
	}

	return nil

}
