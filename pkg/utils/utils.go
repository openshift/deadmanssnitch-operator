package utils

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/openshift/deadmanssnitch-operator/config"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var log = logf.Log.WithName("")

// HasFinalizer returns true if the given object has the given finalizer
func HasFinalizer(object metav1.Object, finalizer string) bool {
	finalizers := sets.NewString(object.GetFinalizers()...)
	return finalizers.Has(finalizer)
}

// AddFinalizer adds a finalizer to the given object
func AddFinalizer(object metav1.Object, finalizer string) {
	finalizers := sets.NewString(object.GetFinalizers()...)
	finalizers.Insert(finalizer)
	object.SetFinalizers(finalizers.List())
}

// DeleteFinalizer removes a finalizer from the given object
func DeleteFinalizer(object metav1.Object, finalizer string) {
	finalizers := sets.NewString(object.GetFinalizers()...)
	finalizers.Delete(finalizer)
	object.SetFinalizers(finalizers.List())
}

// CheckClusterDeployment returns true if the ClusterDeployment is watched by this operator
func CheckClusterDeployment(request reconcile.Request, client client.Client, reqLogger logr.Logger) (bool, *hivev1.ClusterDeployment, error) {

	// remove SyncSetPostfix from name to lookup the ClusterDeployment
	cdName := strings.Replace(request.Name, config.SyncSetPostfix, "", 1)
	cdNamespace := request.Namespace

	clusterDeployment := &hivev1.ClusterDeployment{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: cdName, Namespace: cdNamespace}, clusterDeployment)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("No matching cluster deployment found, ignoring")
			return false, clusterDeployment, nil
		}
		// Error finding the cluster deployment, requeue
		return false, clusterDeployment, err
	}

	if clusterDeployment.DeletionTimestamp != nil {
		return false, clusterDeployment, nil
	}

	if !clusterDeployment.Spec.Installed {
		return false, clusterDeployment, nil
	}

	if val, ok := clusterDeployment.GetLabels()[config.ClusterDeploymentManagedLabel]; ok {
		if val != "true" {
			reqLogger.Info("Is not a managed cluster")
			return false, clusterDeployment, nil
		}
	} else {
		// Managed tag is not present which implies it is not a managed cluster
		reqLogger.Info("Is not a managed cluster")
		return false, clusterDeployment, nil
	}

	// made it this far so it's both managed and has alerts enabled
	return true, clusterDeployment, nil
}

// DeleteSyncSet deletes a SyncSet
func DeleteSyncSet(name string, namespace string, client client.Client) error {
	syncset := &hivev1.SyncSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, syncset)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil
		}
		// Error finding the syncset, requeue
		return err
	}

	// Only delete the syncset, this is just cleanup of the synced secret.
	// The ClusterDeployment controller manages deletion of the deadmanssnitch serivce.
	err = client.Delete(context.TODO(), syncset)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil
		}
		// Error finding the syncset, requeue
		return err
	}

	return nil
}

// DeleteRefSecret deletes Secret which referenced by SyncSet
func DeleteRefSecret(name string, namespace string, client client.Client) error {
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, secret)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil
		}
		// Error finding the secret, requeue
		return err
	}

	// Delete the secret
	log.Info("Deleting Referenced Secret", "Namespace", namespace, "Name", name)
	err = client.Delete(context.TODO(), secret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return nil
		}
		// Error finding the secret, requeue
		return err
	}

	return nil
}

func SecretName(clusterName, optionalPostFix string) string {
	secretName := clusterName + "-" + config.RefSecretPostfix
	if optionalPostFix != "" {
		secretName = clusterName + "-" + optionalPostFix + "-" + config.RefSecretPostfix
	}
	return secretName
}
