package deadmanssnitch

import (
	"context"

	"github.com/openshift/deadmanssnitch-operator/pkg/dmsclient"
	hivev1alpha1 "github.com/openshift/hive/pkg/apis/hive/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	//"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	//"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_deadmanssnitch")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DeadMansSnitch Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	// get dms key
	dmsAPIKey := "TESTAPIKEY"
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
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDeadMansSnitch) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DeadMansSnitch")

	// Fetch the DeadMansSnitch instance
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

	reqLogger.Info("Creating a new SyncSet", "Namespace", request.Namespace, "Name", request.Name)
	newSS := newSyncSet(request.Namespace, request.Name)
	r.client.Create(context.TODO(), newSS)

	// Define a new Pod object
	/*
		pod := newPodForCR(instance)

		// Set DeadMansSnitch instance as the owner and controller
		if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
			return reconcile.Result{}, err
		}

		// Check if this Pod already exists
		found := &corev1.Pod{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
			err = r.client.Create(context.TODO(), pod)
			if err != nil {
				return reconcile.Result{}, err
			}

			// Pod created successfully - don't requeue
			return reconcile.Result{}, nil
		} else if err != nil {
			return reconcile.Result{}, err
		}
	*/

	// Pod already exists - don't requeue
	//reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil

}

func newSyncSet(namespace string, name string) *hivev1alpha1.SyncSet {

	newSS := &hivev1alpha1.SyncSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testSS",
			Namespace: namespace,
		},
		Spec: hivev1alpha1.SyncSetSpec{
			ClusterDeploymentRefs: []corev1.LocalObjectReference{
				{
					Name: name,
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
								"API_KEY": []byte("FIXME: Get PD from vault then generate on API"),
							},
						},
					},
				},
			},
		},
	}

	return newSS

}

/*
// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *hivev1alpha1.DeadMansSnitch) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}
*/
