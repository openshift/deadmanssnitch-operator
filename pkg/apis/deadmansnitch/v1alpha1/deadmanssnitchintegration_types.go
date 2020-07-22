package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeadmansSnitchIntegrationSpec defines the desired state of DeadmansSnitchIntegration
type DeadmansSnitchIntegrationSpec struct {
	DmsAPIKeySecretRef corev1.SecretReference `json:"dmsAPIKeySecretRef"`

	ClusterDeploymentSelector metav1.LabelSelector `json:"clusterdeploymentSelector"`

	TargetSecretRef corev1.SecretReference `json:"targetSecretRef"`

	Tags  []string `json:"tags"`

	SnitchNamepostFix string `json:"snitchNamePostFix"`

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
