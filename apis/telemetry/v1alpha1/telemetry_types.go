// Types in this package are instantiated in the other types in k8ssandra-operator, especially Stargate types and Cassandra types.
package v1alpha1

type TelemetrySpec struct {
	Prometheus *PrometheusTelemetrySpec `json:"prometheus,omitempty"`
}

type PrometheusTelemetrySpec struct {
	// Enable the creation of Prometheus serviceMonitors for this resource (Cassandra or Stargate).
	Enabled *bool `json:"enabled,omitempty"`
	// CommonLabels are applied to all serviceMonitors created.
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
	// Telemetry namespace is the namespace you want the ServiceMonitors to be created in.
	TelemetryNamespace *string `json:"telemetryNamespace,omitempty"`
}
