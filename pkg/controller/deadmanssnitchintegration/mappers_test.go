// Copyright 2020 Red Hat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deadmanssnitchintegration

import (
	"testing"

	deadmanssnitchapis "github.com/openshift/deadmanssnitch-operator/pkg/apis"
	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/pkg/apis/deadmansnitch/v1alpha1"
	hiveapis "github.com/openshift/hive/pkg/apis"
	hivev1 "github.com/openshift/hive/pkg/apis/hive/v1"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestClusterDeploymentToDeadMansSnitchIntegrationsMapper(t *testing.T) {
	deadmanssnitchapis.AddToScheme(scheme.Scheme)
	hiveapis.AddToScheme(scheme.Scheme)

	tests := []struct {
		name             string
		mapper           func(client client.Client) handler.Mapper
		objects          []runtime.Object
		mapObject        handler.MapObject
		expectedRequests []reconcile.Request
	}{
		{
			name:             "clusterDeploymentToDeadMansSnitchIntegrations: empty",
			mapper:           clusterDeploymentToDeadMansSnitchIntegrations,
			objects:          []runtime.Object{},
			mapObject:        handler.MapObject{},
			expectedRequests: []reconcile.Request{},
		},
		{
			name:   "clusterDeploymentToDeadMansSnitchIntegrations: two matching Deadmansnitchintegrations, one not matching",
			mapper: clusterDeploymentToDeadMansSnitchIntegrations,
			objects: []runtime.Object{
				deadMansSnitchIntegration("test1", map[string]string{"test": "test"}),
				deadMansSnitchIntegration("test2", map[string]string{"test": "test"}),
				deadMansSnitchIntegration("test3", map[string]string{"notmatching": "test"}),
			},
			mapObject: handler.MapObject{
				Meta: &metav1.ObjectMeta{
					Labels: map[string]string{"test": "test"},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "test1",
						Namespace: "test",
					},
				},
				{
					NamespacedName: types.NamespacedName{
						Name:      "test2",
						Namespace: "test",
					},
				},
			},
		},

		{
			name:    "ownedByClusterDeploymentToDeadMansSnitchIntegrations: empty",
			mapper:  ownedByClusterDeploymentToDeadMansSnitchIntegrations,
			objects: []runtime.Object{},
			mapObject: handler.MapObject{
				Meta: &metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: hivev1.SchemeGroupVersion.String(),
						Kind:       "ClusterDeployment",
						Name:       "test",
						UID:        types.UID("test"),
					}},
				},
			},
			expectedRequests: []reconcile.Request{},
		},
		{
			name:   "ownedByClusterDeploymentToDeadMansSnitchIntegrations: matched by 3 PagerDutyIntegrations",
			mapper: ownedByClusterDeploymentToDeadMansSnitchIntegrations,
			objects: []runtime.Object{
				deadMansSnitchIntegration("test1", map[string]string{"test": "test"}),
				deadMansSnitchIntegration("test2", map[string]string{"test": "test"}),
				deadMansSnitchIntegration("test3", map[string]string{"test": "test"}),
				deadMansSnitchIntegration("test4", map[string]string{"notmatching": "test"}),
				clusterDeployment("cd1", map[string]string{"test": "test"}),
			},
			mapObject: handler.MapObject{
				Meta: &metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: hivev1.SchemeGroupVersion.String(),
						Kind:       "ClusterDeployment",
						Name:       "cd1",
						UID:        types.UID("test"),
					}},
				},
			},
			expectedRequests: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      "test1",
						Namespace: "test",
					},
				},
				{
					NamespacedName: types.NamespacedName{
						Name:      "test2",
						Namespace: "test",
					},
				},
				{
					NamespacedName: types.NamespacedName{
						Name:      "test3",
						Namespace: "test",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewFakeClient(test.objects...)
			mapper := test.mapper(client)

			actualRequests := mapper.Map(test.mapObject)

			assert.Equal(t, test.expectedRequests, actualRequests)
		})
	}
}

func clusterDeploymentToDeadMansSnitchIntegrations(client client.Client) handler.Mapper {
	return clusterDeploymentToDeadMansSnitchIntegrationsMapper{Client: client}
}

func ownedByClusterDeploymentToDeadMansSnitchIntegrations(client client.Client) handler.Mapper {
	return ownedByClusterDeploymentToDeadMansSnitchIntegrationsMapper{Client: client}
}

func deadMansSnitchIntegration(name string, labels map[string]string) *deadmanssnitchv1alpha1.DeadmansSnitchIntegration {
	return &deadmanssnitchv1alpha1.DeadmansSnitchIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Spec: deadmanssnitchv1alpha1.DeadmansSnitchIntegrationSpec{
			ClusterDeploymentSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			DmsAPIKeySecretRef: v1.SecretReference{
				Name:      "test",
				Namespace: "test",
			},
			TargetSecretRef: v1.SecretReference{
				Name:      "test",
				Namespace: "test",
			},
		},
	}
}

func clusterDeployment(name string, labels map[string]string) *hivev1.ClusterDeployment {
	return &hivev1.ClusterDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
			Labels:    labels,
		},
		Spec: hivev1.ClusterDeploymentSpec{
			ClusterName: name,
			Installed:   true,
		},
	}
}
