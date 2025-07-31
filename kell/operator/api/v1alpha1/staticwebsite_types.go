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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StaticWebsiteSpec defines the desired state of a StaticWebsite.
// This is what a user will provide in their YAML.
type StaticWebsiteSpec struct {
	// The git repository URL for the static website's content.
	// +kubebuilder:validation:Required
	GitRepo string `json:"gitRepo"`

	// The number of replicas to run for the website.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas"`
}

// StaticWebsiteStatus defines the observed state of StaticWebsite.
// This is what our operator will update to report back.
type StaticWebsiteStatus struct {
	// A human-readable status message.
	// +optional
	Message string `json:"message,omitempty"`

	// The number of available replicas.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// StaticWebsite is the Schema for the staticwebsites API
type StaticWebsite struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StaticWebsiteSpec   `json:"spec,omitempty"`
	Status StaticWebsiteStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// StaticWebsiteList contains a list of StaticWebsite
type StaticWebsiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StaticWebsite `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StaticWebsite{}, &StaticWebsiteList{})
}
