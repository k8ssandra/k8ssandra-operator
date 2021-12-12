// package v1alpha1: Types in this package are instantiated in the other types in k8ssandra-operator, especially Stargate types and Cassandra types.
//+kubebuilder:object:generate=true
package v1alpha1

type TelemetrySpec struct {
	Prometheus *PrometheusTelemetrySpec `json:"prometheus,omitempty"`
}
type PrometheusTelemetrySpec struct {
	// Enable the creation of Prometheus serviceMonitors for this resource (Cassandra or Stargate).
	Enabled bool `json:"enabled,omitempty"` // A bool flag required here to disambiguate when e.g. the cluster should have telemetry turned on but one DC should have it explicitly turned off.
	// CommonLabels are applied to all serviceMonitors created.
	// +optional
	CommonLabels map[string]string `json:"commonLabels,omitempty"`
}
