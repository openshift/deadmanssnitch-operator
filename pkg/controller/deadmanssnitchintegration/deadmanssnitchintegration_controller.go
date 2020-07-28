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
	// references, queue a request for all DeadMansSnitchIntegration CR that
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
	// references, queue a request for all DeadMansSnitchIntegration CR that
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
		err = r.createSyncset(dmsi, clustDeploy)
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

// Add finalizers to both the deadmanssnitch integreation and the matching cluster deployment
func (r *ReconcileDeadmansSnitchIntegration) dmsAddFinalizer(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, clustDeploy *hivev1.ClusterDeployment) error {
	var DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch-" + dmsi.Name
	logger := log.WithValues("DeadMansSnitchIntegreation.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", clustDeploy.Name, "cluster-deployment.Namespace:", clustDeploy.Namespace)
	//checking i finalizers exits in the clusterdeployment adding if they dont
	logger.Info("Checking for finalizers")
	if utils.HasFinalizer(clustDeploy, DeadMansSnitchFinalizer) == false {
		log.Info(fmt.Sprint("Adding finalizer to clusterDeployment Name:  ", clustDeploy.Name+" namespace:"+clustDeploy.Namespace+" DMSI Name  :"+dmsi.Name))
		utils.AddFinalizer(clustDeploy, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), clustDeploy)
		if err != nil {
			return err
		}

	}
	logger.Info("Cluster deployment finalizer already exists Name: ")

	//checking i finalizers exits in the dmsi cr adding if they dont
	logger.Info("Checking for finalizers")
	if utils.HasFinalizer(dmsi, DeadMansSnitchFinalizer) == false {
		log.Info(fmt.Sprint("Adding finalizer to DMSI Name: ", " DMSI Name: :"+dmsi.Name))
		utils.AddFinalizer(dmsi, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), dmsi)
		if err != nil {
			return err
		}

	}
	logger.Info("DMSI finalizer already exists: ")

	return nil

}

// create snitch in deadmanssnitch.com with information retrived from dmsi cr as well as the matching cluster deployment
func (r *ReconcileDeadmansSnitchIntegration) createSnitch(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, cd *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {
	logger := log.WithValues("DeadMansSnitchIntegreation.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)
	logger.Info("Checking if snitch already exits Name: ")
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
		logger.Info("Creating snitch Name: ")
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
			logger.Info("Checking in Snitch ...")
			// CheckIn snitch
			err = dmsc.CheckIn(snitch)
			if err != nil {
				logger.Error(err, "Unable to check in deadman's snitch", "CheckInURL", snitch.CheckInURL)
				return err
			}
		}
	} else {
		logger.Error(err, "Unable to get Snitch by name")
		return err
	}
	logger.Info("Snitch found already exists ")
	return nil
}

//Create secret containing the snitch url
func (r *ReconcileDeadmansSnitchIntegration) createSecret(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, dmsc dmsclient.Client, cd hivev1.ClusterDeployment) error {
	logger := log.WithValues("DeadMansSnitchIntegreation.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)
	dmsSecret := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	logger.Info("Checking if secret already exits")
	err := r.client.Get(context.TODO(),
		types.NamespacedName{Name: dmsSecret, Namespace: cd.Namespace},
		&corev1.Secret{})
	if errors.IsNotFound(err) {
		logger.Info("Secret not found creating secret")
		snitchName := cd.Spec.ClusterName + "." + cd.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
		ReSnitches, err := dmsc.FindSnitchesByName(snitchName)

		if err != nil {
			return err
		}
		for _, CheckInURL := range ReSnitches {

			newdmsSecret := newDMSSecret(cd.Namespace, dmsSecret, CheckInURL.CheckInURL)

			// set the owner reference about the secret for gabage collection
			if err := controllerutil.SetControllerReference(&cd, newdmsSecret, r.scheme); err != nil {
				logger.Error(err, "Error setting controller reference on secret")
				return err
			}
			// Create the secret
			if err := r.client.Create(context.TODO(), newdmsSecret); err != nil {
				logger.Error(err, "Failed to create secret")
				return err
			}

		}
	}
	logger.Info("Secret Already Present, nothing to do here...")
	return nil
}

//creating the syncset which contain the secret with the snitch url
func (r *ReconcileDeadmansSnitchIntegration) createSyncset(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, cd hivev1.ClusterDeployment) error {
	logger := log.WithValues("DeadMansSnitchIntegreation.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)
	dmsSecret := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	ssName := cd.Spec.ClusterName + "-" + dmsi.Spec.SnitchNamePostFix + "-" + "dms-secret"
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ssName, Namespace: cd.Namespace}, &hivev1.SyncSet{})

	if errors.IsNotFound(err) {
		logger.Info("SyncSet not found, Creating a new SyncSet")

		newSS := newSyncSet(cd.Namespace, dmsSecret, cd.Name, dmsi)
		if err := controllerutil.SetControllerReference(&cd, newSS, r.scheme); err != nil {
			logger.Error(err, "Error setting controller reference on syncset")
			return err
		}
		if err := r.client.Create(context.TODO(), newSS); err != nil {
			logger.Error(err, "Error creating syncset")
			return err
		}
		logger.Info("Done creating a new SyncSet")

	} else {
		logger.Info("SyncSet Already Present, nothing to do here...")
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

// delete snitches,secrets and syncset associated with the dmsi that has been deleted
func (r *ReconcileDeadmansSnitchIntegration) deleteDMSI(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, clustDeploy *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {
	logger := log.WithValues("DeadMansSnitchIntegreation.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", clustDeploy.Name, "cluster-deployment.Namespace:", clustDeploy.Namespace)
	var DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch-" + dmsi.Name

	// Delete the dms
	logger.Info("Deleting the DMS from api.deadmanssnitch.com")
	snitchName := clustDeploy.Spec.ClusterName + "." + clustDeploy.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
	snitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	for _, s := range snitches {
		delStatus, err := dmsc.Delete(s.Token)
		if !delStatus || err != nil {
			logger.Info("Failed to delete the DMS from api.deadmanssnitch.com")
			return err
		}
		logger.Info("Deleted the DMS from api.deadmanssnitch.com")
	}

	// Delete the SyncSet
	logger.Info("Deleting DMS SyncSet")
	err = utils.DeleteSyncSet(dmsi.Name+config.SyncSetPostfix, dmsi.Namespace, r.client)
	if err != nil {
		logger.Error(err, "Error deleting SyncSet")
		return err
	}

	// Delete the referenced secret
	logger.Info("Deleting DMS referenced secret")
	err = utils.DeleteRefSecret(dmsi.Name+config.RefSecretPostfix, dmsi.Namespace, r.client)
	if err != nil {
		logger.Error(err, "Error deleting secret")
		return err
	}

	logger.Info("Deleting DMS finalizer from dmsi")
	if utils.HasFinalizer(dmsi, DeadMansSnitchFinalizer) {
		utils.DeleteFinalizer(dmsi, DeadMansSnitchFinalizer)
		err = r.client.Update(context.TODO(), dmsi)
		if err != nil {
			logger.Error(err, "Error deleting Finalizer from dmsi")
			return err
		}

		return nil
	}
	return nil

}

// delete snitches,secrets and syncset associated with the cluster deployment that has been deleted
func (r *ReconcileDeadmansSnitchIntegration) deleteDMSClusterDeploy(dmsi *deadmansnitchv1alpha1.DeadmansSnitchIntegration, clustDeploy *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {

	var DeadMansSnitchFinalizer string = "dms.managed.openshift.io/deadmanssnitch-" + dmsi.Name
	logger := log.WithValues("DeadMansSnitchIntegreation.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", clustDeploy.Name, "cluster-deployment.Namespace:", clustDeploy.Namespace)
	// Delete the dms
	logger.Info("Deleting the DMS from api.deadmanssnitch.com")
	snitchName := clustDeploy.Spec.ClusterName + "." + clustDeploy.Spec.BaseDomain + "-" + dmsi.Spec.SnitchNamePostFix
	snitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	for _, s := range snitches {
		delStatus, err := dmsc.Delete(s.Token)
		if !delStatus || err != nil {
			logger.Info("Failed to delete the DMS from api.deadmanssnitch.com")
			return err
		}
		logger.Info("Deleted the DMS from api.deadmanssnitch.com")
	}

	// Delete the SyncSet
	logger.Info("Deleting DMS SyncSet")
	err = utils.DeleteSyncSet(clustDeploy.Name+"-"+dmsi.Spec.SnitchNamePostFix+config.RefSecretPostfix, clustDeploy.Namespace, r.client)
	if err != nil {
		logger.Error(err, "Error deleting SyncSet")
		return err
	}

	// Delete the referenced secret
	logger.Info("Deleting DMS referenced secret")
	err = utils.DeleteRefSecret(clustDeploy.Name+"-"+dmsi.Spec.SnitchNamePostFix+config.RefSecretPostfix, clustDeploy.Namespace, r.client)
	if err != nil {
		logger.Error(err, "Error deleting secret")
		return err
	}

	logger.Info("Deleting DMS finalizer from clusterdeploy")

	utils.DeleteFinalizer(clustDeploy, DeadMansSnitchFinalizer)
	err = r.client.Update(context.TODO(), clustDeploy)
	if err != nil {
		logger.Error(err, "Error deleting Finalizer from clustdeploy")
		return err
	}

	return nil

}
