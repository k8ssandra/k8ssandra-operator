package v1alpha1

import (
	"encoding/json"
)

// +kubebuilder:validation:Type=object
type Unstructured map[string]interface{}

func (u *Unstructured) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	m := map[string]interface{}(*u)
	return json.Marshal(m)
}

func (u *Unstructured) UnmarshalJSON(b []byte) error {
	if b == nil {
		return nil
	}
	m := map[string]interface{}(*u)
	err := json.Unmarshal(b, &m)
	*u = m
	return err
}

func (u *Unstructured) DeepCopy() *Unstructured {
	out := new(Unstructured)
	*out = *u
	return out
}
