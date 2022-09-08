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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeadmansSnitchIntegrationSpec defines the desired state of DeadmansSnitchIntegration
type DeadmansSnitchIntegrationSpec struct {
	//reference to the secret containing deadmanssnitch-api-key
	DmsAPIKeySecretRef corev1.SecretReference `json:"dmsAPIKeySecretRef"`

	//a label selector used to find which clusterdeployment CRs receive a DMS integration based on this configuration
	ClusterDeploymentSelector metav1.LabelSelector `json:"clusterDeploymentSelector"`

	//a list of annotations the operator to skip
	ClusterDeploymentAnnotationsToSkip []ClusterDeploymentAnnotationsToSkip `json:"clusterDeploymentAnnotationsToSkip,omitempty"`

	//name and namespace in the target cluster where the secret is synced
	TargetSecretRef corev1.SecretReference `json:"targetSecretRef"`

	//Array of strings that are applied to the service created in DMS
	Tags []string `json:"tags,omitempty"`

	//The postfix to append to any snitches managed by this integration.  I.e. "osd" or "rhmi"
	SnitchNamePostFix string `json:"snitchNamePostFix,omitempty"`
}

// DeadmansSnitchIntegrationStatus defines the observed state of DeadmansSnitchIntegration
type DeadmansSnitchIntegrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=deadmanssnitchintegrations,shortName=dmsi,scope=Namespaced

// DeadmansSnitchIntegration is the Schema for the deadmanssnitchintegrations API
type DeadmansSnitchIntegration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeadmansSnitchIntegrationSpec   `json:"spec"`
	Status DeadmansSnitchIntegrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DeadmansSnitchIntegrationList contains a list of DeadmansSnitchIntegration
type DeadmansSnitchIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeadmansSnitchIntegration `json:"items"`
}

// ClusterDeploymentAnnotationsToSkip contains a list of annotation keys and values
// The operator will skip the cluster deployment if it has the same annotations set
type ClusterDeploymentAnnotationsToSkip struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func init() {
	SchemeBuilder.Register(&DeadmansSnitchIntegration{}, &DeadmansSnitchIntegrationList{})
}
