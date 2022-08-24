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

	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/api/v1alpha1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &enqueueRequestForClusterDeployment{}

// enqueueRequestForClusterDeployment implements the handler.EventHandler interface.
// Heavily inspired by https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/handler/enqueue_mapped.go
type enqueueRequestForClusterDeployment struct {
	Client client.Client
}

func (e *enqueueRequestForClusterDeployment) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForClusterDeployment) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.ObjectOld, reqs)
	e.mapAndEnqueue(q, evt.ObjectNew, reqs)
}

func (e *enqueueRequestForClusterDeployment) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

func (e *enqueueRequestForClusterDeployment) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]struct{}{}
	e.mapAndEnqueue(q, evt.Object, reqs)
}

// toRequests receives a ClusterDeployment objects that have fired an event and checks if it can find an associated
// DeadmansSnitchIntegration object that has a matching label selector, if so it creates a request for the reconciler to
// take a look at that DeadmansSnitchIntegration object.
func (e *enqueueRequestForClusterDeployment) toRequests(obj client.Object) []reconcile.Request {
	reqs := []reconcile.Request{}
	dmiList := &deadmanssnitchv1alpha1.DeadmansSnitchIntegrationList{}
	if err := e.Client.List(context.TODO(), dmiList, &client.ListOptions{}); err != nil {
		return reqs
	}

	for _, dmi := range dmiList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&dmi.Spec.ClusterDeploymentSelector)
		if err != nil {
			continue
		}
		if selector.Matches(labels.Set(obj.GetLabels())) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      dmi.Name,
					Namespace: dmi.Namespace,
				},
			})
		}
	}
	return reqs
}

func (e *enqueueRequestForClusterDeployment) mapAndEnqueue(q workqueue.RateLimitingInterface, obj client.Object, reqs map[reconcile.Request]struct{}) {
	for _, req := range e.toRequests(obj) {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			// Used for de-duping requests
			reqs[req] = struct{}{}
		}
	}
}

var _ handler.EventHandler = &enqueueRequestForClusterDeploymentOwner{}

// enqueueRequestForClusterDeploymentOwner implements the handler.EventHandler interface.
// Heavily inspired by https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.1/pkg/handler/enqueue_mapped.go
type enqueueRequestForClusterDeploymentOwner struct {
	Client    client.Client
	Scheme    *runtime.Scheme
	groupKind schema.GroupKind
}

func (e *enqueueRequestForClusterDeploymentOwner) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestForClusterDeploymentOwner) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.ObjectOld)
	e.mapAndEnqueue(q, evt.ObjectNew)
}

func (e *enqueueRequestForClusterDeploymentOwner) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestForClusterDeploymentOwner) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestForClusterDeploymentOwner) getClusterDeploymentGroupKind() {
	e.groupKind = schema.GroupKind{
		Group: hivev1.HiveAPIGroup,
		Kind:  "ClusterDeployment",
	}
}

// getAssociatedDeadmansSnitchIntegrations receives objects and checks if they're owned by a ClusterDeployment. If so, it then
// collects associated DeadmansSnitchIntegration CRs and creates requests for the reconciler to consider.
func (e *enqueueRequestForClusterDeploymentOwner) getAssociatedDeadmansSnitchIntegrations(obj metav1.Object) map[reconcile.Request]struct{} {
	e.getClusterDeploymentGroupKind()

	cds := []*hivev1.ClusterDeployment{}
	for _, ref := range obj.GetOwnerReferences() {
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			log.Error(err, "could not parse OwnerReference APIVersion", "api version", ref.APIVersion)
			return map[reconcile.Request]struct{}{}
		}

		if ref.Kind == e.groupKind.Kind && refGV.Group == e.groupKind.Group {
			cd := &hivev1.ClusterDeployment{}
			if err := e.Client.Get(context.TODO(), client.ObjectKey{Namespace: obj.GetNamespace(), Name: ref.Name}, cd); err != nil {
				log.Error(err, "could not get ClusterDeployment", "namespace", obj.GetNamespace(), "name", ref.Name)
				continue
			}
			cds = append(cds, cd)
		}
	}

	if len(cds) == 0 {
		return map[reconcile.Request]struct{}{}
	}

	reqs := map[reconcile.Request]struct{}{}
	dmiList := &deadmanssnitchv1alpha1.DeadmansSnitchIntegrationList{}
	if err := e.Client.List(context.TODO(), dmiList, &client.ListOptions{}); err != nil {
		log.Error(err, "could not list DeadmansSnitchIntegrations")
		return reqs
	}

	for _, dmi := range dmiList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&dmi.Spec.ClusterDeploymentSelector)
		if err != nil {
			log.Error(err, "could not build ClusterDeployment label selector")
			continue
		}
		for _, cd := range cds {
			if selector.Matches(labels.Set(cd.ObjectMeta.GetLabels())) {
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dmi.Name,
						Namespace: dmi.Namespace,
					},
				}
				reqs[request] = struct{}{}
			}
		}
	}

	return reqs
}

func (e *enqueueRequestForClusterDeploymentOwner) mapAndEnqueue(q workqueue.RateLimitingInterface, obj client.Object) {
	for req := range e.getAssociatedDeadmansSnitchIntegrations(obj) {
		q.Add(req)
	}
}
