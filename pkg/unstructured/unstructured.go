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

// Get returns the value at the given path and a boolean indicating whether the path exists. Path
// segments should be separated by slashes, as in: foo/bar/qix.  If this method is invoked on a nil
// receiver, nil and false is returned. If the map is nil, or if the path does not exist, nil and
// false is returned.
func (in *Unstructured) Get(path string) (interface{}, bool) {
	if in == nil {
		return nil, false
	}
	if *in == nil {
		return nil, false
	}
	keys := strings.Split(path, PathSeparator)
	return utils.GetMapNested(*in, keys[0], keys[1:]...)
}

// Put puts the value at the given path. Path segments should be separated by slashes, as in:
// foo/bar/qix. If this method is invoked on a nil receiver, it is a noop. If the map is nil, or if
// any intermediary key does not exist, it is instantiated.
func (in *Unstructured) Put(path string, v interface{}) {
	if in == nil {
		return
	}
	if *in == nil {
		*in = make(Unstructured)
	}
	keys := strings.Split(path, PathSeparator)
	_ = utils.PutMapNested(true, *in, v, keys[0], keys[1:]...)
}

// PutIfAbsent puts the value at the given path, if and only if the path does not exist, or exists
// but holds a nil value. Path segments should be separated by slashes, as in: foo/bar/qix. If this
// method is invoked on a nil receiver, it is a noop. If the map is nil, or if any intermediary key
// does not exist, it is instantiated.
func (in *Unstructured) PutIfAbsent(path string, v interface{}) {
	if in == nil {
		return
	}
	if *in == nil {
		*in = make(Unstructured)
	}
	keys := strings.Split(path, PathSeparator)
	_ = utils.PutMapNested(false, *in, v, keys[0], keys[1:]...)
}

// PutAll puts the given map in this map, recursively. If this method is invoked on a nil receiver,
// it is a noop. If the map is nil, or if any intermediary key does not exist, it is instantiated.
func (in *Unstructured) PutAll(m map[string]interface{}) {
	if in == nil {
		return
	}
	if *in == nil {
		*in = make(Unstructured)
	}
	*in, _ = utils.MergeMapNested(true, *in, m)
}
