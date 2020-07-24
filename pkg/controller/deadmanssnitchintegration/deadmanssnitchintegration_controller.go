package deadmanssnitchintegration

import (
	"context"
	"fmt"

	"github.com/openshift/deadmanssnitch-operator/config"
	deadmansnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/pkg/apis/deadmansnitch/v1alpha1"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	"github.com/openshift/deadmanssnitch-operator/pkg/utils"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// DeadMansSnitchOperatorNamespace is the namespace where this operator will run
	DeadMansSnitchOperatorNamespace string = "deadmanssnitch-operator"
	// DeadMansSnitchAPISecretName is the secret Name where to fetch the DMS API Key
	DeadMansSnitchAPISecretName string = "deadmanssnitch-api-key"
	// DeadMansSnitchAPISecretKey is the secret where to fetch the DMS API Key
	DeadMansSnitchAPISecretKey string = "deadmanssnitch-api-key"
)

var log = logf.Log.WithName("controller_deadmanssnitchintegration")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DeadmansSnitchIntegration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	newRec := newReconciler(mgr)
	return add(mgr, newRec)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDeadmansSnitchIntegration{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deadmanssnitchintegration-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DeadmansSnitchIntegration
	err = c.Watch(&source.Kind{Type: &deadmansnitchv1alpha1.DeadmansSnitchIntegration{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// // TODO(user): Modify this to be the types you create that are owned by the primary resource
	// // Watch for changes to secondary resource Pods and requeue the owner DeadmansSnitchIntegration
	// err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	// 	IsController: true,
	// 	OwnerType:    &deadmansnitchv1alpha1.DeadmansSnitchIntegration{},
	// })
	// if err != nil {
	// 	return err
	// }

	return nil
}

// blank assignment to verify that ReconcileDeadmansSnitchIntegration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDeadmansSnitchIntegration{}

// ReconcileDeadmansSnitchIntegration reconciles a DeadmansSnitchIntegration object
type ReconcileDeadmansSnitchIntegration struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DeadmansSnitchIntegration object and makes changes based on the state read
// and what is in the DeadmansSnitchIntegration.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDeadmansSnitchIntegration) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DeadmansSnitchIntegration")
	// Fetch the DeadmansSnitchIntegration dmsi
	dmsi := &deadmansnitchv1alpha1.DeadmansSnitchIntegration{}
	err := r.client.Get(context.TODO(), request.NamespacedName, dmsi)
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

	dmsAPIKey, err := utils.LoadSecretData(r.client, dmsi.Spec.DmsAPIKeySecretRef.Name,
		dmsi.Spec.DmsAPIKeySecretRef.Namespace, DeadMansSnitchAPISecretKey)
	if err != nil {
		return reconcile.Result{}, err
	}
	dmsc := dmsclient.NewClient(dmsAPIKey)

	matchingClusterDeployments, err := r.getMatchingClusterDeployment(dmsi)
	if err != nil {
		return reconcile.Result{}, err
	}

	// if dmsi.DeletionTimestamp == nil {
	// 	if utils.HasFinalizer(dmsi, DeadMansSnitchFinalizer) {
	// 		for _, clustDeploy := range matchingClusterDeployments.Items {
	// 			// delete dmsi func
	// 			reqLogger.Info("Hello world test", clustDeploy)
	// 		}
	// 	}

	// }
	for _, clustDeploy := range matchingClusterDeployments.Items {
		err = r.dmsAddFinalizer(dmsi, &clustDeploy)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.createSnitch(dmsi, &clustDeploy, dmsc)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.createSecretAndSyncset(dmsi, dmsc, clustDeploy)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, err
}

func (r *ReconcileDeadmansSnitchIntegration) getMatchingClusterDeployment(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration) (*hivev1.ClusterDeploymentList, error) {

	labelSelector := dmsi.Spec.ClusterDeploymentSelector.DeepCopy()
	labelSelector.MatchExpressions = append(labelSelector.MatchExpressions, metav1.LabelSelectorRequirement{
		Key:      config.ClusterDeploymentNoalertsLabel,
		Operator: metav1.LabelSelectorOpNotIn,
		Values:   []string{"true"},
	})
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	matchingClusterDeployments := &hivev1.ClusterDeploymentList{}
	listOpts := &client.ListOptions{LabelSelector: selector}
	err = r.client.List(context.TODO(), matchingClusterDeployments, listOpts)
	return matchingClusterDeployments, err
}

func (r *ReconcileDeadmansSnitchIntegration) dmsAddFinalizer(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, clustDeploy *hivev1.ClusterDeployment) error {
	var DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch-" + dmsi.Name
	if utils.HasFinalizer(clustDeploy, DeadMansSnitchFinalizer) == false {
		log.Info("Adding finalizer to clusterDeployment:")
		utils.AddFinalizer(clustDeploy, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), clustDeploy)
		if err != nil {
			return err
		}

	}

	return nil

}

func (r *ReconcileDeadmansSnitchIntegration) createSnitch(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, cd *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {

	snitchName := cd.Spec.ClusterName + "." + cd.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
	snitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	var snitch dmsclient.Snitch
	if len(snitches) > 0 {
		snitch = snitches[0]
	} else {
		newSnitch := dmsclient.NewSnitch(snitchName, dmsi.Spec.Tags, "15_minute", "basic")
		snitch, err = dmsc.Create(newSnitch)
		if err != nil {
			return err
		}
	}
	ReSnitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}

	if len(ReSnitches) > 0 {
		if ReSnitches[0].Status == "pending" {
			log.Info("Checking in Snitch ...", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
			// CheckIn snitch
			err = dmsc.CheckIn(snitch)
			if err != nil {
				log.Error(err, "Unable to check in deadman's snitch", "Namespace", dmsi.Namespace, "Name", dmsi.Name, "CheckInURL", snitch.CheckInURL)
				return err
			}
		}
	} else {
		log.Error(err, "Unable to get Snitch by name", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
		return err
	}
	return nil
}

func (r *ReconcileDeadmansSnitchIntegration) createSecretAndSyncset(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, dmsc dmsclient.Client, cd hivev1.ClusterDeployment) error {
	dmsSecret := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	log.Info("SECRET")
	log.Info(dmsSecret)
	err := r.client.Get(context.TODO(),
		types.NamespacedName{Name: dmsSecret, Namespace: dmsi.Namespace},
		&corev1.Secret{})
	if errors.IsNotFound(err) {
		snitchName := cd.Spec.ClusterName + "." + cd.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
		ReSnitches, err := dmsc.FindSnitchesByName(snitchName)
		log.Info(fmt.Sprintf("SNITCHES %v", ReSnitches))

		if err != nil {
			return err
		}
		for _, CheckInURL := range ReSnitches {

			log.Info(CheckInURL.Name)
			newdmsSecret := newDMSSecret(dmsi.Namespace, dmsSecret, CheckInURL.CheckInURL)

			// set the owner reference about the secret for gabage collection
			if err := controllerutil.SetControllerReference(dmsi, newdmsSecret, r.scheme); err != nil {
				log.Error(err, "Error setting controller refernce on secret", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
				return err
			}
			// Create the secret
			if err := r.client.Create(context.TODO(), newdmsSecret); err != nil {
				log.Error(err, "Failed to create secret", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
				return err
			}

		}

		ssName := dmsi.Name + config.SyncSetPostfix
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: ssName, Namespace: dmsi.Namespace}, &hivev1.SyncSet{})

		if errors.IsNotFound(err) {
			log.Info("SyncSet not found, Creating a new SynsSet", "Namespace", dmsi.Namespace, "Name", dmsi.Name)

			newSS := newSyncSet(dmsi.Namespace, dmsSecret, dmsi.Name)
			if err := controllerutil.SetControllerReference(dmsi, newSS, r.scheme); err != nil {
				log.Error(err, "Error setting controller reference on syncset", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
				return err
			}
			if err := r.client.Create(context.TODO(), newSS); err != nil {
				log.Error(err, "Error creating syncset", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
				return err
			}
			log.Info("Done creating a new SyncSet", "Namespace", dmsi.Namespace, "Name", dmsi.Name)

		} else {
			log.Info("SyncSet Already Present, nothing to do here...", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
			// return directly if the syscset already existed
			return nil
		}

	}

	return nil
}

func newDMSSecret(namespace string, name string, snitchURL string) *corev1.Secret {

	dmsSecret := &corev1.Secret{
		Type: "Opaque",
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			config.KeySnitchURL: []byte(snitchURL),
		},
	}

	return dmsSecret

}

func newSyncSet(namespace string, dmsSecret string, clusterDeploymentName string) *hivev1.SyncSet {

	newSS := &hivev1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dmsSecret,
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
							Name:      dmsSecret,
							Namespace: namespace,
						},
						TargetRef: hivev1.SecretReference{
							Name:      dmsSecret,
							Namespace: "openshift-monitoring",
						},
					},
				},
			},
		},
	}

	return newSS

}
