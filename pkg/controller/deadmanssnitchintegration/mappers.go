package deadmanssnitchintegration

import (
	"context"
	"strings"

	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/pkg/apis/deadmanssnitch/v1alpha1"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type clusterDeploymentToDeadMansSnitchIntegrationsMapper struct {
	Client client.Client
}

func (m clusterDeploymentToDeadMansSnitchIntegrationsMapper) Map(mo handler.MapObject) []reconcile.Request {
	dmsilist := &deadmanssnitchv1alpha1.DeadmansSnitchIntegrationList{}
	err := m.Client.List(context.TODO(), dmsilist, &client.ListOptions{})
	if err != nil {
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}
	for _, dmsi := range dmsilist.Items {
		selector, err := metav1.LabelSelectorAsSelector(&dmsi.Spec.ClusterDeploymentSelector)
		if err != nil {
			logrus.Debug(err)
			continue
		}
		if selector.Matches(labels.Set(mo.Meta.GetLabels())) {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      dmsi.Name,
					Namespace: dmsi.Namespace,
				}},
			)
		}
	}
	return requests
}

type ownedByClusterDeploymentToDeadMansSnitchIntegrationsMapper struct {
	Client client.Client
}

func (m ownedByClusterDeploymentToDeadMansSnitchIntegrationsMapper) Map(mo handler.MapObject) []reconcile.Request {
	relevantClusterDeployments := []*hivev1.ClusterDeployment{}
	for _, or := range mo.Meta.GetOwnerReferences() {
		if or.APIVersion == hivev1.SchemeGroupVersion.String() && strings.ToLower(or.Kind) == "clusterdeployment" {
			cd := &hivev1.ClusterDeployment{}
			err := m.Client.Get(context.TODO(), client.ObjectKey{Name: or.Name, Namespace: mo.Meta.GetNamespace()}, cd)
			if err != nil {
				logrus.Debug(err)
				continue
			}
			relevantClusterDeployments = append(relevantClusterDeployments, cd)
		}
	}
	if len(relevantClusterDeployments) == 0 {
		return []reconcile.Request{}
	}

	dmsilist := &deadmanssnitchv1alpha1.DeadmansSnitchIntegrationList{}
	err := m.Client.List(context.TODO(), dmsilist, &client.ListOptions{})
	if err != nil {
		return []reconcile.Request{}
	}

	requests := []reconcile.Request{}
	for _, dmi := range dmsilist.Items {
		selector, err := metav1.LabelSelectorAsSelector(&dmi.Spec.ClusterDeploymentSelector)
		if err != nil {
			logrus.Debug(err)
			continue
		}

		for _, cd := range relevantClusterDeployments {
			if selector.Matches(labels.Set(cd.ObjectMeta.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      dmi.Name,
						Namespace: dmi.Namespace,
					}},
				)
			}
		}
	}
	return requests
}
