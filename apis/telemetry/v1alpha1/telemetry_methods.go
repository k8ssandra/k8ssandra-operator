package v1alpha1

// Merge takes an object a and merges another object, b's values into it, overwriting any which conflict.
func (a *TelemetrySpec) Merge(b *TelemetrySpec) *TelemetrySpec {
	// TODO: This method is brittle. It must be updated whenever any field is added to `TelemetrySpec``. It
	// would be best to replace these methods with a generic client side `Merge()` based on the k8s strategic merge
	// patch logic.
	out := TelemetrySpec{}
	if a == nil && b == nil {
		return nil
	} else if b == nil {
		return a
	} else if a == nil {
		return b
	}
	if b.Prometheus == nil {
		out.Prometheus = a.Prometheus
	} else {
		out.Prometheus = a.Prometheus.Merge(b.Prometheus)
	}
	return &out
}

// Merge takes an object a and merges another object, b's values into it, overwriting any which conflict.
func (a *PrometheusTelemetrySpec) Merge(b *PrometheusTelemetrySpec) *PrometheusTelemetrySpec {
	// TODO: This method is brittle. It must be updated whenever any field is added to `PrometheusTelemetrySpec`. It
	// would be best to replace these methods with a generic client side `Merge()` based on the k8s strategic merge
	// patch logic.
	out := PrometheusTelemetrySpec{}
	if a == nil && b == nil {
		return nil
	} else if b == nil {
		return a
	} else if a == nil {
		return b
	}
	if a.CommonLabels != nil {
		for k, v := range a.CommonLabels {
			out.CommonLabels[k] = v
		}
	}
	if b.CommonLabels != nil {
		for k, v := range b.CommonLabels {
			out.CommonLabels[k] = v
		}
	}
	out.Enabled = b.Enabled
	return &out
}
