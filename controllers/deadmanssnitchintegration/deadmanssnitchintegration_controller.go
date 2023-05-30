/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deadmanssnitchintegration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/api/v1alpha1"
	"github.com/openshift/deadmanssnitch-operator/config"
	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	"github.com/openshift/deadmanssnitch-operator/pkg/localmetrics"
	"github.com/openshift/deadmanssnitch-operator/pkg/utils"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logf.Log.WithName("controller_deadmanssnitchintegration")

const (
	deadMansSnitchAPISecretKey    = "deadmanssnitch-api-key"
	DeadMansSnitchFinalizerPrefix = "dms.managed.openshift.io/deadmanssnitch-"
)

// DeadmansSnitchIntegrationReconciler reconciles a DeadmansSnitchIntegration object
type DeadmansSnitchIntegrationReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	Client    client.Client
	Scheme    *runtime.Scheme
	dmsclient func(authToken string, collector *localmetrics.MetricsCollector) dmsclient.Client
}

//+kubebuilder:rbac:groups=deadmanssnitch.managed.openshift.io,resources=deadmanssnitchintegrations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deadmanssnitch.managed.openshift.io,resources=deadmanssnitchintegrations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deadmanssnitch.managed.openshift.io,resources=deadmanssnitchintegrations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DeadmansSnitchIntegration object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *DeadmansSnitchIntegrationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {

	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DeadmansSnitchIntegration")

	if config.IsFedramp() {
		reqLogger.Info("Running in FedRAMP mode")
	}

	// Fetch the DeadmansSnitchIntegration dmsi
	dmsi := &deadmanssnitchv1alpha1.DeadmansSnitchIntegration{}

	err := r.Client.Get(context.TODO(), request.NamespacedName, dmsi)
	if err != nil {
		if k8errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// set the DMS finalizer variable
	deadMansSnitchFinalizer := DeadMansSnitchFinalizerPrefix + dmsi.Name

	dmsAPIKey, err := utils.LoadSecretData(r.Client, dmsi.Spec.DmsAPIKeySecretRef.Name,
		dmsi.Spec.DmsAPIKeySecretRef.Namespace, deadMansSnitchAPISecretKey)
	if err != nil {
		return reconcile.Result{}, err
	}
	dmsc := r.dmsclient(dmsAPIKey, localmetrics.Collector)

	matchingClusterDeployments, err := r.getMatchingClusterDeployment(dmsi)
	if err != nil {
		return reconcile.Result{}, err
	}

	allClusterDeployments, err := r.getAllClusterDeployment()
	if err != nil {
		return reconcile.Result{}, err
	}

	if dmsi.DeletionTimestamp != nil {
		for i := range allClusterDeployments.Items {
			clusterdeployment := allClusterDeployments.Items[i]
			if utils.HasFinalizer(&clusterdeployment, deadMansSnitchFinalizer) {
				err = r.deleteDMSClusterDeployment(dmsi, &clusterdeployment, dmsc)
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
		if utils.HasFinalizer(dmsi, deadMansSnitchFinalizer) {
			utils.DeleteFinalizer(dmsi, deadMansSnitchFinalizer)
			reqLogger.Info("Deleting DMSI finalizer from dmsi", "DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name)
			err = r.Client.Update(context.TODO(), dmsi)
			if err != nil {
				reqLogger.Error(err, "Error deleting Finalizer from dmsi")
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	for i := range allClusterDeployments.Items {
		clusterdeployment := allClusterDeployments.Items[i]
		// Check if the cluster matches the requirements for needing DMS setup
		clusterMatched := false
		for _, matchingClusterDeployment := range matchingClusterDeployments {
			if clusterdeployment.UID == matchingClusterDeployment.UID {
				clusterMatched = true
				break
			}
		}

		if !clusterMatched || clusterdeployment.DeletionTimestamp != nil {
			// The cluster does not match the criteria for needing DMS setup
			if utils.HasFinalizer(&clusterdeployment, deadMansSnitchFinalizer) {
				// The cluster has an existing DMS setup, so remove it
				err = r.deleteDMSClusterDeployment(dmsi, &clusterdeployment, dmsc)
				if err != nil {
					return reconcile.Result{}, err
				}
			}
			continue
		}

		if !clusterdeployment.Spec.Installed {
			// The cluster isn't installed yet, so don't setup DMS yet either
			continue
		}

		err = r.dmsAddFinalizer(dmsi, &clusterdeployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		secretExist, syncSetExist, err := r.snitchResourcesExist(dmsi, &clusterdeployment)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Check if the cluster is hibernating
		specIsHibernating := clusterdeployment.Spec.PowerState == hivev1.ClusterPowerStateHibernating
		if specIsHibernating {
			if secretExist || syncSetExist {
				err := r.deleteDMSClusterDeployment(dmsi, &clusterdeployment, dmsc)
				if err != nil {
					return reconcile.Result{}, err
				}

			}
		} else {
			// If the cluster is a new install or if the cluster is not hibernating
			// create DMS resources
			if !secretExist || !syncSetExist {
				err = r.createSnitch(dmsi, &clusterdeployment, dmsc)
				if err != nil {
					return reconcile.Result{}, err
				}

				err = r.createSecret(dmsi, dmsc, clusterdeployment)
				if err != nil {
					return reconcile.Result{}, err
				}

				err = r.createSyncset(dmsi, clusterdeployment)
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}

	log.Info("Reconcile of deadmanssnitch integration complete")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeadmansSnitchIntegrationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dmsclient = dmsclient.NewClient
	return ctrl.NewControllerManagedBy(mgr).
		For(&deadmanssnitchv1alpha1.DeadmansSnitchIntegration{}).
		Watches(&source.Kind{Type: &hivev1.ClusterDeployment{}}, &enqueueRequestForClusterDeployment{
			Client: mgr.GetClient(),
		}).
		Watches(&source.Kind{Type: &hivev1.SyncSet{}}, &enqueueRequestForClusterDeploymentOwner{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &enqueueRequestForClusterDeploymentOwner{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}).Complete(r)
}

// getMatchingClusterDeployment gets all ClusterDeployments matching the DMSI selector
func (r *DeadmansSnitchIntegrationReconciler) getMatchingClusterDeployment(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration) ([]hivev1.ClusterDeployment, error) {
	selector, err := metav1.LabelSelectorAsSelector(&dmsi.Spec.ClusterDeploymentSelector)
	if err != nil {
		return nil, err
	}

	matchingClusterDeployments := &hivev1.ClusterDeploymentList{}
	listOpts := &client.ListOptions{LabelSelector: selector}
	err = r.Client.List(context.TODO(), matchingClusterDeployments, listOpts)

	matchedClusterDeployments := []hivev1.ClusterDeployment{}

	// If the ClusterDeploymentAnnotationsToSkip set in the DMS integration
	// Check the cluster deployment and skip it if the annotation has the same
	// key and value
	if len(dmsi.Spec.ClusterDeploymentAnnotationsToSkip) > 0 {
		for _, cd := range matchingClusterDeployments.Items {
			if !r.shouldSkipClusterDeployment(dmsi.Spec.ClusterDeploymentAnnotationsToSkip, cd) {
				matchedClusterDeployments = append(matchedClusterDeployments, cd)
			}
		}
	} else {
		matchedClusterDeployments = matchingClusterDeployments.Items
	}

	return matchedClusterDeployments, err
}

func (r *DeadmansSnitchIntegrationReconciler) shouldSkipClusterDeployment(clusterDeploymentAnnotationsToSkip []deadmanssnitchv1alpha1.ClusterDeploymentAnnotationsToSkip, cd hivev1.ClusterDeployment) bool {
	for annoKey, annoVal := range cd.GetAnnotations() {
		for _, skipper := range clusterDeploymentAnnotationsToSkip {
			if annoKey == skipper.Name && annoVal == skipper.Value {
				return true
			}
		}
	}
	return false
}

// getAllClusterDeployment retrives all ClusterDeployments in the shard
func (r *DeadmansSnitchIntegrationReconciler) getAllClusterDeployment() (*hivev1.ClusterDeploymentList, error) {
	matchingClusterDeployments := &hivev1.ClusterDeploymentList{}
	err := r.Client.List(context.TODO(), matchingClusterDeployments, &client.ListOptions{})
	return matchingClusterDeployments, err
}

// Add finalizers to both the deadmanssnitch integration and the matching cluster deployment
func (r *DeadmansSnitchIntegrationReconciler) dmsAddFinalizer(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration, clusterdeployment *hivev1.ClusterDeployment) error {
	deadMansSnitchFinalizer := DeadMansSnitchFinalizerPrefix + dmsi.Name
	logger := log.WithValues("DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", clusterdeployment.Name, "cluster-deployment.Namespace:", clusterdeployment.Namespace)
	//checking i finalizers exits in the clusterdeployment adding if they dont
	logger.Info("Checking for finalizers")
	if !utils.HasFinalizer(clusterdeployment, deadMansSnitchFinalizer) {
		log.Info(fmt.Sprint("Adding finalizer to cluster Deployment Name:  ", clusterdeployment.Name+" namespace:"+clusterdeployment.Namespace+" DMSI Name  :"+dmsi.Name))
		baseToPatch := client.MergeFrom(clusterdeployment.DeepCopy())
		utils.AddFinalizer(clusterdeployment, deadMansSnitchFinalizer)
		if err := r.Client.Patch(context.TODO(), clusterdeployment, baseToPatch); err != nil {
			return err
		}
	}
	logger.V(1).Info("Cluster deployment finalizer created nothing to do here ...: ")

	//checking i finalizers exits in the dmsi cr adding if they dont
	logger.V(1).Info("Checking for finalizers")
	if !utils.HasFinalizer(dmsi, deadMansSnitchFinalizer) {
		log.Info(fmt.Sprint("Adding finalizer to DMSI Name: ", " DMSI Name: :"+dmsi.Name))
		utils.AddFinalizer(dmsi, deadMansSnitchFinalizer)
		err := r.Client.Update(context.TODO(), dmsi)
		if err != nil {
			return err
		}

	}
	logger.V(1).Info("DMSI finalizer created nothing to do here..: ")

	return nil

}

// create snitch in deadmanssnitch.com with information retrived from dmsi cr as well as the matching cluster deployment
func (r *DeadmansSnitchIntegrationReconciler) createSnitch(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration, cd *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {
	logger := log.WithValues("DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)

	clusterID, err := getClusterID(*cd, config.IsFedramp())
	if err != nil {
		return err
	}
	snitchName := getSnitchName(*cd, dmsi.Spec.SnitchNamePostFix, config.IsFedramp())
	ssName := utils.SecretName(cd.Spec.ClusterName, dmsi.Spec.SnitchNamePostFix)

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: ssName, Namespace: cd.Namespace}, &hivev1.SyncSet{})
	if err != nil {
		if k8errors.IsNotFound(err) {
			logger.V(1).Info(fmt.Sprint("Checking if snitch already exists SnitchName:", snitchName))
			snitches, err := dmsc.FindSnitchesByName(snitchName)
			if err != nil {
				return err
			}

			if len(snitches) <= 0 {
				newSnitch := dmsclient.NewSnitch(snitchName, dmsi.Spec.Tags, "15_minute", "basic")
				newSnitch.Notes = fmt.Sprintf("cluster_id: %s\nrunbook: https://github.com/openshift/ops-sop/blob/master/v4/alerts/cluster_has_gone_missing.md", clusterID)
				// add escaping since _ is not being recognized otherwise.
				newSnitch.Notes = "```" + newSnitch.Notes + "```"
				logger.Info(fmt.Sprint("Creating snitch:", snitchName))
				_, err = dmsc.Create(newSnitch)
				if err != nil {
					return err
				}
			}

			ReSnitches, err := dmsc.FindSnitchesByName(snitchName)
			if err != nil {
				return err
			}

			if len(ReSnitches) <= 0 {
				logger.Error(err, "Unable to get Snitch by name")
				return err
			} else {
				for _, sn := range ReSnitches {
					if sn.Status == "pending" {
						logger.Info("Checking in Snitch ", sn.CheckInURL)
						err = dmsc.CheckIn(sn)
						if err != nil {
							logger.Error(err, "Unable to check in deadman's snitch", "CheckInURL", sn.CheckInURL)
							return err
						}
					}
				}
			}
		}
	}

	logger.V(1).Info("Snitch created nothing to do here.... ")
	return nil
}

// snitchResourcesExist checks if the associated cluster resources for a snitch exist
func (r *DeadmansSnitchIntegrationReconciler) snitchResourcesExist(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration, cd *hivev1.ClusterDeployment) (bool, bool, error) {
	logger := log.WithValues("DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)
	logger.Info("Checking for snitch resources")
	dmsSecret := utils.SecretName(cd.Spec.ClusterName, dmsi.Spec.SnitchNamePostFix)
	logger.Info("Checking if secret exists")
	secretExist := false
	err := r.Client.Get(context.TODO(),
		types.NamespacedName{Name: dmsSecret, Namespace: cd.Namespace},
		&corev1.Secret{})
	if err != nil && !k8errors.IsNotFound(err) {
		return false, false, err
	}
	secretExist = !k8errors.IsNotFound(err)

	logger.Info("Checking if syncset exists")
	syncSetExist := false
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: dmsSecret, Namespace: cd.Namespace}, &hivev1.SyncSet{})
	if err != nil && !k8errors.IsNotFound(err) {
		return secretExist, false, err
	}
	syncSetExist = !k8errors.IsNotFound(err)

	return secretExist, syncSetExist, nil
}

// Create secret containing the snitch url
func (r *DeadmansSnitchIntegrationReconciler) createSecret(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration, dmsc dmsclient.Client, cd hivev1.ClusterDeployment) error {
	logger := log.WithValues("DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)
	dmsSecret := utils.SecretName(cd.Spec.ClusterName, dmsi.Spec.SnitchNamePostFix)
	logger.V(1).Info("Checking if secret already exits")
	err := r.Client.Get(context.TODO(),
		types.NamespacedName{Name: dmsSecret, Namespace: cd.Namespace},
		&corev1.Secret{})

	if err != nil && !k8errors.IsNotFound(err) {
		logger.Error(err, "Failed to fetch secret")
		return err
	}

	if k8errors.IsNotFound(err) {
		logger.Info("Secret not found creating secret")
		snitchName := getSnitchName(cd, dmsi.Spec.SnitchNamePostFix, config.IsFedramp())
		ReSnitches, err := dmsc.FindSnitchesByName(snitchName)

		if err != nil {
			return err
		}
		for _, CheckInURL := range ReSnitches {

			newdmsSecret := newDMSSecret(cd.Namespace, dmsSecret, CheckInURL.CheckInURL)

			// set the owner reference about the secret for gabage collection
			if err := controllerutil.SetControllerReference(&cd, newdmsSecret, r.Scheme); err != nil {
				logger.Error(err, "Error setting controller reference on secret")
				return err
			}
			// Create the secret
			if err := r.Client.Create(context.TODO(), newdmsSecret); err != nil {
				logger.Error(err, "Failed to create secret")
				return err
			}

		}
	}
	logger.V(1).Info("Secret created, nothing to do here...")
	return nil
}

// creating the syncset which contain the secret with the snitch url
func (r *DeadmansSnitchIntegrationReconciler) createSyncset(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration, cd hivev1.ClusterDeployment) error {
	logger := log.WithValues("DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", cd.Name, "cluster-deployment.Namespace:", cd.Namespace)
	ssName := utils.SecretName(cd.Spec.ClusterName, dmsi.Spec.SnitchNamePostFix)
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ssName, Namespace: cd.Namespace}, &hivev1.SyncSet{})

	if k8errors.IsNotFound(err) {
		logger.Info("SyncSet not found, Creating a new SyncSet")

		newSS := newSyncSet(cd.Namespace, ssName, cd.Name, dmsi)
		if err := controllerutil.SetControllerReference(&cd, newSS, r.Scheme); err != nil {
			logger.Error(err, "Error setting controller reference on syncset")
			return err
		}
		if err := r.Client.Create(context.TODO(), newSS); err != nil {
			logger.Error(err, "Error creating syncset")
			return err
		}
		logger.Info("Done creating a new SyncSet")

	} else {
		logger.V(1).Info("SyncSet Already Present, nothing to do here...")
		// return directly if the syscset already existed

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

func newSyncSet(namespace string, dmsSecret string, clusterDeploymentName string, dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration) *hivev1.SyncSet {

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

// delete snitches,secrets and syncset associated with the cluster deployment that has been deleted
func (r *DeadmansSnitchIntegrationReconciler) deleteDMSClusterDeployment(dmsi *deadmanssnitchv1alpha1.DeadmansSnitchIntegration, clusterDeployment *hivev1.ClusterDeployment, dmsc dmsclient.Client) error {
	deadMansSnitchFinalizer := DeadMansSnitchFinalizerPrefix + dmsi.Name
	logger := log.WithValues("DeadMansSnitchIntegration.Namespace", dmsi.Namespace, "DMSI.Name", dmsi.Name, "cluster-deployment.Name:", clusterDeployment.Name, "cluster-deployment.Namespace:", clusterDeployment.Namespace)

	// Delete the dms
	logger.Info("Deleting the DMS from api.deadmanssnitch.com")
	snitchName := getSnitchName(*clusterDeployment, dmsi.Spec.SnitchNamePostFix, config.IsFedramp())
	snitches, err := dmsc.FindSnitchesByName(snitchName)
	if err != nil {
		return err
	}
	for _, s := range snitches {
		delStatus, err := dmsc.Delete(s.Token)
		if !delStatus || err != nil {
			logger.Error(err, "Failed to delete the DMS from api.deadmanssnitch.com")
			return err
		}
		logger.Info("Deleted the DMS from api.deadmanssnitch.com")
	}

	// Delete the SyncSet
	logger.Info("Deleting DMS SyncSet")
	dmsSecret := utils.SecretName(clusterDeployment.Spec.ClusterName, dmsi.Spec.SnitchNamePostFix)
	err = utils.DeleteSyncSet(dmsSecret, clusterDeployment.Namespace, r.Client)
	if err != nil {
		logger.Error(err, "Error deleting SyncSet")
		return err
	}

	// Delete the referenced secret
	logger.Info("Deleting DMS referenced secret")
	err = utils.DeleteRefSecret(dmsSecret, clusterDeployment.Namespace, r.Client)
	if err != nil {
		logger.Error(err, "Error deleting secret")
		return err
	}

	if utils.HasFinalizer(clusterDeployment, deadMansSnitchFinalizer) {
		logger.Info("Deleting DMSI finalizer from cluster deployment")
		baseToPatch := client.MergeFrom(clusterDeployment.DeepCopy())
		utils.DeleteFinalizer(clusterDeployment, deadMansSnitchFinalizer)
		if err := r.Client.Patch(context.TODO(), clusterDeployment, baseToPatch); err != nil {
			logger.Error(err, "Error deleting Finalizer from cluster deployment")
			return err
		}
	}

	return nil

}

// getClusterID determines if fedramp or not
// Returns internal clusterID for fedramp and external clusterID if not
func getClusterID(cd hivev1.ClusterDeployment, isFedramp bool) (string, error) {
	if cd.Spec.ClusterMetadata == nil || cd.Spec.ClusterMetadata.ClusterID == "" {
		return "", errors.New("Unable to get ClusterID from ClusterDeployment")
	}

	clusterID := cd.Spec.ClusterMetadata.ClusterID

	if isFedramp {
		clusterID = getInternalClusterID(cd)
	}
	return clusterID, nil
}

// getSnitchName determines if fedramp or not
// Returns internal clusterID for fedramp and "(cd.Spec.ClusterName).(cd.Spec.BaseDomain)" if not
func getSnitchName(cd hivev1.ClusterDeployment, optionalPostFix string, isFedramp bool) string {
	snitchName := cd.Spec.ClusterName + "." + cd.Spec.BaseDomain
	if optionalPostFix != "" {
		snitchName += "-" + optionalPostFix
	}

	if isFedramp {
		snitchName = getInternalClusterID(cd)
	}
	return snitchName
}

func getInternalClusterID(cd hivev1.ClusterDeployment) string {
	cns := strings.Split(cd.Namespace, "-")
	return cns[len(cns)-1]
}
