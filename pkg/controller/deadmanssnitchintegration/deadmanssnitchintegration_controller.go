package deadmanssnitchintegration

import (
	"context"
	"fmt"

	deadmansnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/pkg/apis/deadmansnitch/v1alpha1"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	"github.com/openshift/deadmanssnitch-operator/pkg/utils"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	"github.com/openshift/pagerduty-operator/config"

	// corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	// "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// DeadMansSnitchFinalizer is used on ClusterDeployments to ensure we run a successful deprovision
	// job before cleaning up the API object.
	
	// DeadMansSnitchOperatorNamespace is the namespace where this operator will run
	DeadMansSnitchOperatorNamespace string = "deadmanssnitch-operator"
	// DeadMansSnitchAPISecretName is the secret Name where to fetch the DMS API Key
	DeadMansSnitchAPISecretName string = "deadmanssnitch-api-key"
	// DeadMansSnitchAPISecretKey is the secret where to fetch the DMS API Key
	DeadMansSnitchAPISecretKey string = "deadmanssnitch-api-key"
	// DeadMansSnitchTagKey is the secret where to fetch the DMS API Key
	DeadMansSnitchTagKey string = "hive-cluster-tag"
)

var log = logf.Log.WithName("controller_deadmanssnitchintegration")
/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DeadmansSnitchIntegration Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	newRec, err := newReconciler(mgr)
	if err != nil {
		return err
	}
	return add(mgr, newRec)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	tempClient, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return nil, err
	}
	dmsiAPIKey, err := utils.LoadSecretData(tempClient, DeadMansSnitchAPISecretName,
		DeadMansSnitchOperatorNamespace, DeadMansSnitchAPISecretKey)
	if err != nil {
		return nil, err
	}

	return &ReconcileDeadmansSnitchIntegration{

		client:    mgr.GetClient(),
		scheme:    mgr.GetScheme(),
		dmsclient: dmsclient.NewClient(dmsiAPIKey)}, nil
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
	client    client.Client
	scheme    *runtime.Scheme
	dmsclient dmsclient.Client
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
		err	=	r.dmsAddFinalizer(dmsi, &clustDeploy)
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
	if utils.HasFinalizer(clustDeploy, DeadMansSnitchFinalizer) == false{
		log.Info("Adding finalizer to clusterDeployment:")
		
		utils.AddFinalizer(clustDeploy, DeadMansSnitchFinalizer)
		err := r.client.Update(context.TODO(), clustDeploy)
		if err != nil {
			// log.Error(err, "Error setting Finalizer for ClusterDeployment" ,clustDeployID)
			return  err
		}
		matchingClusterDeploymentsTest := &hivev1.ClusterDeployment{}
		err = r.client.Get(context.TODO(),types.NamespacedName{Name: clustDeploy.Name,Namespace: clustDeploy.Namespace},matchingClusterDeploymentsTest)
		if err != nil{
			return err
		}
		log.Info(fmt.Sprintf("matchclustTest%v",matchingClusterDeploymentsTest))


	}

	//log.Info("DMS Finalizer already exists in clusterDeployment")
	return nil

}

// func (r *ReconcileDeadmansSnitchIntegration) createDMSSecret(request reconcile.Request) (reconcile.Result,error){
// 	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

// 	err := r.client.Get(context.TODO(),types.NamespacedName{Name: refSecretName, Namespace: request.Namespace},&corev1.Secret{})
// 	if err != nil {
// 		return reconcile.Result{},err
// 	}
// 	reqLogger.Info("Checking for secret, Secret not found, Creating a new Secret", "Namespace", request.Namespace, "Name", request.Name)

// }
