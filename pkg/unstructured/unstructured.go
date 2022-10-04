package unstructured

import (
	"encoding/json"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

// Unstructured is a map[string]interface{} that can be used to represent unstructured JSON content.
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
	in.DeepCopyInto(out)
	return out
}

func (in *Unstructured) DeepCopyInto(out *Unstructured) {
	*out = runtime.DeepCopyJSON(*in)
}

const PathSeparator = "/"

func (in *Unstructured) GetNested(path string) (interface{}, bool) {
	if in == nil {
		return nil, false
	}
	if *in == nil {
		return nil, false
	}
	keys := strings.Split(path, PathSeparator)
	return utils.GetMapNested(*in, keys[0], keys[1:]...)
}

func (in *Unstructured) PutNested(path string, v interface{}) {
	if in == nil {
		return
	}
	if *in == nil {
		*in = make(Unstructured)
	}
	keys := strings.Split(path, PathSeparator)
	_ = utils.PutMapNested(true, *in, v, keys[0], keys[1:]...)
}

func (in *Unstructured) PutNestedIfAbsent(path string, v interface{}) {
	if in == nil {
		return
	}
	if *in == nil {
		*in = make(Unstructured)
	}
	keys := strings.Split(path, PathSeparator)
	_ = utils.PutMapNested(false, *in, v, keys[0], keys[1:]...)
}
