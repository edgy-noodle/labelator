/*
Copyright 2025.

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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ConditionType string

type ConditionReason string

// LabelSpec defines the desired state of Label.
type LabelSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Namespaces defines the namespaces to apply the labels to, accepts '*' wildcard
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Namespaces []string `json:"namespaces"`

	// ExcludedNamespaces defines the namespaces to ignore when using a wildcard selector
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ExcludedNamespaces []string `json:"excludedNamespaces,omitempty"`

	// Labels defines the labels to apply onto the Namespaces
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Labels map[string]string `json:"labels"`

	// CleanupOnDelete decides whether the controller will use finalizers and
	// cleanup labels from the namespaces when the Label resource is removed
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CleanupOnDelete bool `json:"cleanupOnDelete,omitempty"`
}

// LabelStatus defines the observed state of Label.
type LabelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Label.status.conditions.type are: "Reconciled", "Degraded".
	// Label.status.conditions.status are one of True, False, Unknown.
	// Label.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// Label.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Conditions store the status conditions of the Label instances representing its current state
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`

	// ReconciledNamespaces is a list of namespaces that have been successfully reconciled
	// +operator-sdk:csv:customresourcedefinitions:type=status
	ReconciledNamespaces []string `json:"reconciledNamespaces,omitempty"`

	// FailedNamespaces is a list of namespaces that failed to be reconciled
	// +operator-sdk:csv:customresourcedefinitions:type=status
	FailedNamespaces []string `json:"failedNamespaces,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster

// Label is the Schema for the labels API.
type Label struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LabelSpec   `json:"spec,omitempty"`
	Status LabelStatus `json:"status,omitempty"`
}

func (l *Label) SetCondition(t ConditionType, r ConditionReason, status metav1.ConditionStatus, msg string) {
	meta.SetStatusCondition(&l.Status.Conditions, metav1.Condition{
		Type:    string(t),
		Status:  status,
		Reason:  string(r),
		Message: msg,
	})
}

// +kubebuilder:object:root=true

// LabelList contains a list of Label.
type LabelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Label `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Label{}, &LabelList{})
}
