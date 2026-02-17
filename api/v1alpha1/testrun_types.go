/*
Copyright 2026.

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

// GitSource defines where to fetch the test script from
type GitSource struct {
	// URL of the git repository
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Path to the script within the repository
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Revision to checkout (branch, tag, sha)
	// +kubebuilder:default=main
	Revision string `json:"revision,omitempty"`
}

// TestRunSpec defines the desired state of TestRun
type TestRunSpec struct {
	// AppName is the name of the target App CR to test against
	// +kubebuilder:validation:Required
	AppName string `json:"appName"`

	// Script is the inline Lua script to execute
	// +optional
	Script string `json:"script,omitempty"`

	// Git source for the script
	// +optional
	Git *GitSource `json:"git,omitempty"`

	// Timeout for the test execution (default 60s)
	// +kubebuilder:default="60s"
	Timeout string `json:"timeout,omitempty"`
}

// TestRunStatus defines the observed state of TestRun
type TestRunStatus struct {
	// State of the test execution
	// +kubebuilder:validation:Enum=Pending;Running;Passed;Failed;Error
	State string `json:"state,omitempty"`

	// RunnerPod is the name of the pod executing the test
	// +optional
	RunnerPod string `json:"runnerPod,omitempty"`

	// StartTime is when the test started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the test finished
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Result summary or error message
	// +optional
	Result string `json:"result,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Result",type=string,JSONPath=`.status.result`
// +kubebuilder:printcolumn:name="Pod",type=string,JSONPath=`.status.runnerPod`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TestRun is the Schema for the testruns API
type TestRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TestRunSpec   `json:"spec,omitempty"`
	Status TestRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TestRunList contains a list of TestRun
type TestRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TestRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TestRun{}, &TestRunList{})
}
