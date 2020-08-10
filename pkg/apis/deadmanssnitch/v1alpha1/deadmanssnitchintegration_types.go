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
	ClusterDeploymentSelector metav1.LabelSelector `json:"clusterdeploymentSelector"`

	//name and namespace in the target cluster where the secret is synced
	TargetSecretRef corev1.SecretReference `json:"targetSecretRef"`

	//Array of strings that are applied to the service created in DMS
	Tags []string `json:"tags"`

	//The postfix to append to any snitches managed by this integration.  I.e. "osd" or "rhmi"
	SnitchNamePostFix string `json:"snitchNamePostFix"`
}

// DeadmansSnitchIntegrationStatus defines the observed state of DeadmansSnitchIntegration
type DeadmansSnitchIntegrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeadmansSnitchIntegration is the Schema for the deadmanssnitchintegrations API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=deadmanssnitchintegrations,scope=Namespaced
type DeadmansSnitchIntegration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DeadmansSnitchIntegrationSpec   `json:"spec,omitempty"`
	Status DeadmansSnitchIntegrationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeadmansSnitchIntegrationList contains a list of DeadmansSnitchIntegration
type DeadmansSnitchIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DeadmansSnitchIntegration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DeadmansSnitchIntegration{}, &DeadmansSnitchIntegrationList{})
}
