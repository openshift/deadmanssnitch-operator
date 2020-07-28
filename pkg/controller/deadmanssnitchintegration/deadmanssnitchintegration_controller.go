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
	//DeadMansSnitchTagKey for tests
	DeadMansSnitchTagKey string = "testTag"
	//DeadMansSnitchFinalizer for tests
	DeadMansSnitchFinalizer string = "testfinalizer"
)

var log = logf.Log.WithName("controller_deadmanssnitchintegration")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DeadmansSnitchIntegration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDeadmansSnitchIntegration{
		//client:    mgr.GetClient(),
		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		dmsclient: dmsclient.NewClient,
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

	err = c.Watch(&source.Kind{Type: &hivev1.ClusterDeployment{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: clusterDeploymentToDeadMansSnitchIntegrationsMapper{
				Client: mgr.GetClient(),
			},
		},
	)
	if err != nil {
		return err
	}

	// Watch for changes to SyncSets. If one has any ClusterDeployment owner
	// references, queue a request for all PagerDutyIntegration CR that
	// select those ClusterDeployments.
	err = c.Watch(&source.Kind{Type: &hivev1.SyncSet{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: ownedByClusterDeploymentToDeadMansSnitchIntegrationsMapper{
				Client: mgr.GetClient(),
			},
		},
	)
	if err != nil {
		return err
	}

	// Watch for changes to Secrets. If one has any ClusterDeployment owner
	// references, queue a request for all PagerDutyIntegration CR that
	// select those ClusterDeployments.
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: ownedByClusterDeploymentToDeadMansSnitchIntegrationsMapper{
				Client: mgr.GetClient(),
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileDeadmansSnitchIntegration implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDeadmansSnitchIntegration{}

// ReconcileDeadmansSnitchIntegration reconciles a DeadmansSnitchIntegration object
type ReconcileDeadmansSnitchIntegration struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	scheme    *runtime.Scheme
	dmsclient func(apiKey string) dmsclient.Client
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
	dmsc := r.dmsclient(dmsAPIKey)

	matchingClusterDeployments, err := r.getMatchingClusterDeployment(dmsi)
	if err != nil {
		return reconcile.Result{}, err
	}

	if dmsi.DeletionTimestamp != nil {
		for _, clustDeploy := range matchingClusterDeployments.Items {
			err = r.deleteDMSI(dmsi, &clustDeploy, dmsc)
			if err != nil {
				return reconcile.Result{}, err
			}

		}
		return reconcile.Result{}, nil
	}

	for _, clustDeploy := range matchingClusterDeployments.Items {
		if clustDeploy.DeletionTimestamp != nil || clustDeploy.Labels[config.ClusterDeploymentNoalertsLabel] == "true" {
			err = r.deleteDMSClusterDeploy(dmsi, &clustDeploy, dmsc)
			if err != nil {
				return reconcile.Result{}, err
			}
			return reconcile.Result{}, nil

		}

		if !clustDeploy.Spec.Installed {
			// Cluster isn't installed yet, return
			return reconcile.Result{}, nil
		}

		err = r.dmsAddFinalizer(dmsi, &clustDeploy)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.createSnitch(dmsi, &clustDeploy, dmsc)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.createSecret(dmsi, dmsc, clustDeploy)
		if err != nil {
			return reconcile.Result{}, err
		}
		err = r.reconcileSyncset(dmsi, clustDeploy)
		if err != nil {
			return reconcile.Result{}, err
		}

	}
	log.Info("Reconcile of deadmanssnitch integration complete")
	return reconcile.Result{}, nil
}

func (r *ReconcileDeadmansSnitchIntegration) getMatchingClusterDeployment(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration) (*hivev1.ClusterDeploymentList, error) {
	selector, err := metav1.LabelSelectorAsSelector(&dmsi.Spec.ClusterDeploymentSelector)
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
	log.Info("Checking for finalizers")
	if utils.HasFinalizer(clustDeploy, DeadMansSnitchFinalizer) == false {
		log.Info(fmt.Sprint("Adding finalizer to clusterDeployment Name:  ", clustDeploy.Name+" namespace:"+clustDeploy.Namespace+"DMSI Name  :"+dmsi.Name))
		utils.AddFinalizer(clustDeploy, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), clustDeploy)
		if err != nil {
			return err
		}

	}
	log.Info(fmt.Sprint("Cluster deployment finalizer already exists Name: ", clustDeploy.Name+" namespace: "+clustDeploy.Namespace+" DMSI Name:"+dmsi.Name))

	if utils.HasFinalizer(dmsi, DeadMansSnitchFinalizer) == false {
		log.Info(fmt.Sprint("Adding finalizer to DMSI Name: ", " DMSI Name: :"+dmsi.Name))
		utils.AddFinalizer(dmsi, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), dmsi)
		if err != nil {
			return err
		}

	}
	log.Info(fmt.Sprint("DMSI finalizer already exists: ", clustDeploy.Name+" namespace:"+clustDeploy.Namespace+" DMSI Name:"+dmsi.Name))

	return nil

}

func (r *ReconcileDeadmansSnitchIntegration) createSnitch(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, cd *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {
	log.Info(fmt.Sprint("Checking if snitch already exits Name: ", cd.Name+" namespace:"+cd.Namespace+" DMSI Name: :"+dmsi.Name))
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
		log.Info(fmt.Sprint("Creating snitch Name: ", cd.Name+"namespace: "+cd.Namespace+" DMSI Name: :"+dmsi.Name))
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
	log.Info("Snitch found already exists ")
	return nil
}

func (r *ReconcileDeadmansSnitchIntegration) createSecret(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, dmsc dmsclient.Client, cd hivev1.ClusterDeployment) error {
	dmsSecret := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	log.Info(fmt.Sprint("Checking if secret already exits Name:   ", cd.Name+"namespace: "+cd.Namespace+"DMSI Name  :"+dmsi.Name))
	err := r.client.Get(context.TODO(),
		types.NamespacedName{Name: dmsSecret, Namespace: cd.Namespace},
		&corev1.Secret{})
	if errors.IsNotFound(err) {
		log.Info(fmt.Sprint("Secret not found creating secret Name:   ", cd.Name+"namespace: "+cd.Namespace+"DMSI Name  :"+dmsi.Name))
		snitchName := cd.Spec.ClusterName + "." + cd.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
		ReSnitches, err := dmsc.FindSnitchesByName(snitchName)

		if err != nil {
			return err
		}
		for _, CheckInURL := range ReSnitches {

			newdmsSecret := newDMSSecret(cd.Namespace, dmsSecret, CheckInURL.CheckInURL)

			// set the owner reference about the secret for gabage collection
			if err := controllerutil.SetControllerReference(&cd, newdmsSecret, r.scheme); err != nil {
				log.Error(err, "Error setting controller reference on secret", "Namespace", cd.Namespace, "Name", cd.Name)
				return err
			}
			// Create the secret
			if err := r.client.Create(context.TODO(), newdmsSecret); err != nil {
				log.Error(err, "Failed to create secret", "Namespace", cd.Namespace, "Name", cd.Name)
				return err
			}

		}
	}
	return nil
}

func (r *ReconcileDeadmansSnitchIntegration) reconcileSyncset(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, cd hivev1.ClusterDeployment) error {
	dmsSecret := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	ssName := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ssName, Namespace: cd.Namespace}, &hivev1.SyncSet{})

	if errors.IsNotFound(err) {
		log.Info("SyncSet not found, Creating a new SyncSet", "Namespace", cd.Namespace, "Name", cd.Name)

		newSS := newSyncSet(cd.Namespace, dmsSecret, cd.Name, dmsi)
		if err := controllerutil.SetControllerReference(&cd, newSS, r.scheme); err != nil {
			log.Error(err, "Error setting controller reference on syncset", "Namespace", cd.Namespace, "Name", cd.Name)
			return err
		}
		if err := r.client.Create(context.TODO(), newSS); err != nil {
			log.Error(err, "Error creating syncset", "Namespace", cd.Namespace, "Name", dmsi.Name)
			return err
		}
		log.Info("Done creating a new SyncSet", "Namespace", cd.Namespace, "Name", dmsi.Name)

	} else {
		log.Info("SyncSet Already Present, nothing to do here...", "Namespace", cd.Namespace, "Name", dmsi.Name)
		// return directly if the syscset already existed
		return nil
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

func newSyncSet(namespace string, dmsSecret string, clusterDeploymentName string, dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration) *hivev1.SyncSet {

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
				// Use SecretReference here which consume the secret in the cluster namespace,
				// instead of embed the secret in the SyncSet directly
				Secrets: []hivev1.SecretMapping{
					{
						SourceRef: hivev1.SecretReference{
							Name:      dmsSecret,
							Namespace: namespace,
						},
						TargetRef: hivev1.SecretReference{
							Name:      dmsi.Spec.TargetSecretRef.Name,
							Namespace: dmsi.Spec.TargetSecretRef.Namespace,
						},
					},
				},
			},
		},
	}

	return newSS

}

func (r *ReconcileDeadmansSnitchIntegration) deleteDMSI(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, clustDeploy *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {

	var DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch-" + dmsi.Name

	// Delete the dms
	log.Info("Deleting the DMS from api.deadmanssnitch.com", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	snitchName := clustDeploy.Spec.ClusterName + "." + clustDeploy.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
	snitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	for _, s := range snitches {
		delStatus, err := dmsc.Delete(s.Token)
		if !delStatus || err != nil {
			log.Info("Failed to delete the DMS from api.deadmanssnitch.com", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
			return err
		}
		log.Info("Deleted the DMS from api.deadmanssnitch.com", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	}

	// Delete the SyncSet
	log.Info("Deleting DMS SyncSet", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	err = utils.DeleteSyncSet(dmsi.Name+config.SyncSetPostfix, dmsi.Namespace, r.client)
	if err != nil {
		log.Error(err, "Error deleting SyncSet", "Namespace", dmsi.Namespace, "Name", dmsi.Name+config.SyncSetPostfix)
		return err
	}

	// Delete the referenced secret
	log.Info("Deleting DMS referenced secret", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	err = utils.DeleteRefSecret(dmsi.Name+config.RefSecretPostfix, dmsi.Namespace, r.client)
	if err != nil {
		log.Error(err, "Error deleting secret", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
		return err
	}

	log.Info("Deleting DMS finalizer from dmsi", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	if utils.HasFinalizer(dmsi, DeadMansSnitchFinalizer) {
		utils.DeleteFinalizer(dmsi, DeadMansSnitchFinalizer)
		err = r.client.Update(context.TODO(), dmsi)
		if err != nil {
			log.Error(err, "Error deleting Finalizer from dmsi", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
			return err
		}

		return nil
	}
	return nil

}
func (r *ReconcileDeadmansSnitchIntegration) deleteDMSClusterDeploy(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, clustDeploy *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {

	var DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch-" + dmsi.Name

	// Delete the dms
	log.Info("Deleting the DMS from api.deadmanssnitch.com", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	snitchName := clustDeploy.Spec.ClusterName + "." + clustDeploy.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
	snitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	for _, s := range snitches {
		delStatus, err := dmsc.Delete(s.Token)
		if !delStatus || err != nil {
			log.Info("Failed to delete the DMS from api.deadmanssnitch.com", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
			return err
		}
		log.Info("Deleted the DMS from api.deadmanssnitch.com", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	}

	// Delete the SyncSet
	log.Info("Deleting DMS SyncSet", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	err = utils.DeleteSyncSet(clustDeploy.Name+"-"+dmsi.Spec.SnitchNamePostFix+config.RefSecretPostfix, clustDeploy.Namespace, r.client)
	if err != nil {
		log.Error(err, "Error deleting SyncSet", "Namespace", dmsi.Namespace, "Name", dmsi.Name+config.SyncSetPostfix)
		return err
	}

	// Delete the referenced secret
	log.Info("Deleting DMS referenced secret", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
	err = utils.DeleteRefSecret(clustDeploy.Name+"-"+dmsi.Spec.SnitchNamePostFix+config.RefSecretPostfix, clustDeploy.Namespace, r.client)
	if err != nil {
		log.Error(err, "Error deleting secret", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
		return err
	}

	log.Info("Deleting DMS finalizer from clusterdeploy", "Namespace", dmsi.Namespace, "Name", dmsi.Name)

	utils.DeleteFinalizer(clustDeploy, DeadMansSnitchFinalizer)
	err = r.client.Update(context.TODO(), clustDeploy)
	if err != nil {
		log.Error(err, "Error deleting Finalizer from clustdeploy", "Namespace", dmsi.Namespace, "Name", dmsi.Name)
		return err
	}

	return nil

}
