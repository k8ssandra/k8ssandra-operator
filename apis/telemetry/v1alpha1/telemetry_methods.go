package v1alpha1

import "github.com/adutra/goalesce"

// Merge takes an object a and merges another object, b's values into it, overwriting any which conflict.
func (a *TelemetrySpec) Merge(b *TelemetrySpec) *TelemetrySpec {
	coalesced, _ := goalesce.Coalesce(a.DeepCopy(), b.DeepCopy(), goalesce.WithTrileans())
	return coalesced.(*TelemetrySpec)
}
