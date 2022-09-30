package v1alpha1

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/runtime"
)

// Unstructured is a map[string]interface{} that can be used to represent unstructured content.
// +kubebuilder:validation:Type=object
type Unstructured map[string]interface{}

func (in *Unstructured) MarshalJSON() ([]byte, error) {
	if in == nil {
		return []byte("null"), nil
	}
	m := map[string]interface{}(*in)
	return json.Marshal(m)
}

func (in *Unstructured) UnmarshalJSON(b []byte) error {
	if b == nil {
		return nil
	}
	m := map[string]interface{}(*in)
	err := json.Unmarshal(b, &m)
	*in = m
	return err
}

func (in *Unstructured) DeepCopy() *Unstructured {
	if in == nil {
		return nil
	}
	out := new(Unstructured)
	*out = runtime.DeepCopyJSON(*in)
	return out
}

func (in *Unstructured) DeepCopyInto(out *Unstructured) {
	clone := in.DeepCopy()
	*out = *clone
}
