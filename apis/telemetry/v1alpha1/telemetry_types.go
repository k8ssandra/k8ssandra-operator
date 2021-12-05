// Types in this package are instantiated in the other types in k8ssandra-operator, especially Stargate types and Cassandra types.
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Telemetry struct {
	TelemetrySpec   `json:"status,omitempty"`
	TelemetryStatus `json:"status,omitempty"`
}

type TelemetrySpec struct {
	Prometheus PrometheusTelemetrySpec `json:"prometheus,omitempty"`
}

type PrometheusTelemetrySpec struct {
	// Enable the creation of Prometheus serviceMonitors for this resource (Cassandra or Stargate).
	Enabled *bool `json:"enabled,omitempty"`
	// CommonLabels are applied to all serviceMonitors created.
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
	// Telemetry namespace is the namespace you want the ServiceMonitors to be created in.
	TelemetryNamespace *string `json:"telemetryNamespace,omitempty"`
}
type TelemetryConditionType string

const (
	TelemetryReady  TelemetryConditionType = "Ready"
	DependancyError TelemetryConditionType = "DependancyError"
)

type TelemetryStatus struct {
	// +optional
	Conditions []TelemetryCondition `json:"conditions,omitempty"`
}
type TelemetryCondition struct {
	Type   TelemetryConditionType `json:"type"`
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime is the last time the condition transited from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
}
