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
	"testing"

	deadmanssnitchv1alpha1 "github.com/openshift/deadmanssnitch-operator/api/v1alpha1"
	hivev1 "github.com/openshift/hive/apis/hive/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_enqueueRequestForClusterDeployment_toRequests(t *testing.T) {
	scheme := runtime.NewScheme()
	s := runtime.SchemeBuilder{
		corev1.AddToScheme,
		hivev1.AddToScheme,
		deadmanssnitchv1alpha1.AddToScheme,
	}
	assert.Nil(t, s.AddToScheme(scheme))

	tests := []struct {
		name             string
		obj              client.Object
		dmsiObjs         []runtime.Object
		expectedRequests int
	}{
		{
			name:             "empty ClusterDeployment",
			obj:              &hivev1.ClusterDeployment{},
			dmsiObjs:         []runtime.Object{},
			expectedRequests: 0,
		},
		{
			name: "empty ClusterDeployment",
			obj: &hivev1.ClusterDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"dmsiWatching": "clusterDeployment1",
					},
				},
			},
			dmsiObjs: []runtime.Object{
				mockDeadmansSnitchIntegration("dmsi1", map[string]string{"dmsiWatching": "clusterDeployment1"}),
				mockDeadmansSnitchIntegration("dmsi2", map[string]string{"dmsiWatching": "clusterDeployment2"}),
				mockDeadmansSnitchIntegration("dmsi3", map[string]string{"dmsiWatching": "clusterDeployment1"}),
			},
			expectedRequests: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &enqueueRequestForClusterDeployment{
				Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(test.obj).WithRuntimeObjects(test.dmsiObjs...).Build(),
			}
			reqs := e.toRequests(test.obj)
			assert.Equal(t, test.expectedRequests, len(reqs))
		})
	}
}

func Test_enqueueRequestForClusterDeploymentOwner_getAssociatedDeadmansSnitchIntegrations(t *testing.T) {
	scheme := runtime.NewScheme()
	s := runtime.SchemeBuilder{
		corev1.AddToScheme,
		hivev1.AddToScheme,
		deadmanssnitchv1alpha1.AddToScheme,
	}
	assert.Nil(t, s.AddToScheme(scheme))

	tests := []struct {
		name             string
		obj              client.Object
		cdObjs           []runtime.Object
		dmsiObjs         []runtime.Object
		expectedRequests int
	}{
		{
			name:             "Secret with no OwnerReference",
			obj:              &corev1.Secret{},
			cdObjs:           []runtime.Object{},
			dmsiObjs:         []runtime.Object{},
			expectedRequests: 0,
		},
		{
			name: "ClusterDeployment",
			obj: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret1",
					Namespace: "ns1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "hive.openshift.io/v1",
							Kind:       "ClusterDeployment",
							Name:       "clusterDeployment1",
						},
					},
				},
			},
			cdObjs: []runtime.Object{
				&hivev1.ClusterDeployment{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ClusterDeployment",
						APIVersion: "hive.openshift.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clusterDeployment1",
						Namespace: "ns1",
						Labels: map[string]string{
							"dmsiWatching": "clusterDeployment1",
						},
					},
				},
			},
			dmsiObjs: []runtime.Object{
				mockDeadmansSnitchIntegration("dmsi1", map[string]string{"dmsiWatching": "clusterDeployment1"}),
				mockDeadmansSnitchIntegration("dmsi2", map[string]string{"dmsiWatching": "clusterDeployment2"}),
				mockDeadmansSnitchIntegration("dmsi3", map[string]string{"dmsiWatching": "clusterDeployment1"}),
			},
			expectedRequests: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &enqueueRequestForClusterDeploymentOwner{
				Client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(test.obj).
					WithRuntimeObjects(test.dmsiObjs...).
					WithRuntimeObjects(test.cdObjs...).
					Build(),
			}
			reqs := e.getAssociatedDeadmansSnitchIntegrations(test.obj)
			assert.Equal(t, test.expectedRequests, len(reqs))
		})
	}
}

func mockDeadmansSnitchIntegration(name string, labels map[string]string) *deadmanssnitchv1alpha1.DeadmansSnitchIntegration {
	return &deadmanssnitchv1alpha1.DeadmansSnitchIntegration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Spec: deadmanssnitchv1alpha1.DeadmansSnitchIntegrationSpec{
			ClusterDeploymentSelector: metav1.LabelSelector{
				MatchLabels: labels,
			},
			DmsAPIKeySecretRef: corev1.SecretReference{
				Name:      "test",
				Namespace: "test",
			},
			TargetSecretRef: corev1.SecretReference{
				Name:      "test",
				Namespace: "test",
			},
		},
	}
}
