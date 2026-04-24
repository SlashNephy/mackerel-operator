/*
Copyright 2026 SlashNephy.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExternalMonitorSpec defines the desired state of ExternalMonitor
// +kubebuilder:validation:XValidation:rule="!has(self.responseTimeWarning) || has(self.responseTimeDuration)",message="responseTimeDuration is required when responseTimeWarning is set"
// +kubebuilder:validation:XValidation:rule="!has(self.responseTimeCritical) || has(self.responseTimeDuration)",message="responseTimeDuration is required when responseTimeCritical is set"
type ExternalMonitorSpec struct {
	Name    string `json:"name,omitempty"`
	Service string `json:"service,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://.+`
	URL string `json:"url"`
	// +kubebuilder:validation:Enum=GET;POST;PUT;DELETE
	// +kubebuilder:default=GET
	Method string `json:"method,omitempty"`
	// +kubebuilder:validation:Minimum=10
	NotificationInterval *int `json:"notificationInterval,omitempty"`
	// +kubebuilder:validation:Minimum=100
	// +kubebuilder:validation:Maximum=599
	ExpectedStatusCode *int   `json:"expectedStatusCode,omitempty"`
	ContainsString     string `json:"containsString,omitempty"`
	// +kubebuilder:validation:Minimum=1
	ResponseTimeDuration *int `json:"responseTimeDuration,omitempty"`
	// +kubebuilder:validation:Minimum=0
	ResponseTimeWarning *int `json:"responseTimeWarning,omitempty"`
	// +kubebuilder:validation:Minimum=0
	ResponseTimeCritical *int `json:"responseTimeCritical,omitempty"`
	// +kubebuilder:validation:Minimum=0
	CertificationExpirationWarning *int `json:"certificationExpirationWarning,omitempty"`
	// +kubebuilder:validation:Minimum=0
	CertificationExpirationCritical *int `json:"certificationExpirationCritical,omitempty"`
	// +kubebuilder:validation:MaxLength=1900
	Memo string `json:"memo,omitempty"`
}

// ExternalMonitorStatus defines the observed state of ExternalMonitor.
type ExternalMonitorStatus struct {
	MonitorID           string       `json:"monitorID,omitempty"`
	ObservedGeneration  int64        `json:"observedGeneration,omitempty"`
	LastSyncedAt        *metav1.Time `json:"lastSyncedAt,omitempty"`
	LastAppliedHash     string       `json:"lastAppliedHash,omitempty"`
	URL                 string       `json:"url,omitempty"`
	MackerelMonitorName string       `json:"mackerelMonitorName,omitempty"`
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ExternalMonitor is the Schema for the externalmonitors API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
// +kubebuilder:printcolumn:name="MonitorID",type=string,JSONPath=`.status.monitorID`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type ExternalMonitor struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ExternalMonitor
	// +required
	Spec ExternalMonitorSpec `json:"spec"`

	// status defines the observed state of ExternalMonitor
	// +optional
	Status ExternalMonitorStatus `json:"status,omitzero"`
}

// ExternalMonitorList contains a list of ExternalMonitor
// +kubebuilder:object:root=true
type ExternalMonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ExternalMonitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExternalMonitor{}, &ExternalMonitorList{})
}
