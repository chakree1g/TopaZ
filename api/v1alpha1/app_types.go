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

// AppSpec defines the desired state of App
type AppSpec struct {
	// Services defines the stack of microservices
	Services []ServiceSpec `json:"services"`

	// Databases defines database services with schema initialization
	// +optional
	Databases []DatabaseSpec `json:"databases,omitempty"`
}

type ServiceSpec struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Version  string `json:"version"`
	Replicas *int32 `json:"replicas,omitempty"`
	Port     int32  `json:"port"`
	// GrpcPort (optional) for gRPC traffic
	// +optional
	GrpcPort *int32 `json:"grpcPort,omitempty"`
	// EnvVars are additional environment variables to inject
	// +optional
	EnvVars map[string]string `json:"envVars,omitempty"`
}

// DatabaseSpec defines a database service managed by the platform
type DatabaseSpec struct {
	// Name of the database service
	Name string `json:"name"`
	// Image is the container image (e.g. postgres:16-alpine)
	Image string `json:"image"`
	// Port the database listens on
	Port int32 `json:"port"`
	// Credentials for connecting (user, password, dbname)
	Credentials map[string]string `json:"credentials"`
	// InitSQL is inline DDL to run once after the database is ready
	// +optional
	InitSQL string `json:"initSQL,omitempty"`
}

// AppStatus defines the observed state of App.
type AppStatus struct {
	// Conditions represent the latest available observations of an object's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastChecked is the timestamp of the last successful health probe
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`

	// EndpointCount is the number of defined endpoints actively being monitored
	// +optional
	EndpointCount int32 `json:"endpointCount,omitempty"`

	// Health status summary
	// +kubebuilder:validation:Enum=Healthy;Unhealthy;Unknown
	// +optional
	Health string `json:"health,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.targetURL`
// +kubebuilder:printcolumn:name="Env",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Health",type=string,JSONPath=`.status.health`
// +kubebuilder:printcolumn:name="Endpoints",type=integer,JSONPath=`.status.endpointCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// App is the Schema for the apps API
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppList contains a list of App
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}
