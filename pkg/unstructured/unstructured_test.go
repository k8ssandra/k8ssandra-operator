package unstructured

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnstructured_MarshalJSON(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name    string
		in      *Unstructured
		want    []byte
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "nil receiver",
			in:      nil,
			want:    []byte(`null`),
			wantErr: assert.NoError,
		},
		{
			name:    "nil map",
			in:      &nilMap,
			want:    []byte(`null`),
			wantErr: assert.NoError,
		},
		{
			name:    "empty",
			in:      &Unstructured{},
			want:    []byte(`{}`),
			wantErr: assert.NoError,
		},
		{
			name: "simple",
			in: &Unstructured{
				"foo": "bar",
			},
			want:    []byte(`{"foo":"bar"}`),
			wantErr: assert.NoError,
		},
		{
			name: "complex",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": 123,
				},
			},
			want:    []byte(`{"foo":{"bar":123}}`),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.in.MarshalJSON()
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, err)
		})
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, err)
		})
	}
}

func TestUnstructured_UnmarshalJSON(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name    string
		b       []byte
		want    *Unstructured
		wantErr assert.ErrorAssertionFunc
	}{
		// nil or empty []byte input not allowed
		{
			name:    "null",
			b:       []byte(`null`),
			want:    &nilMap,
			wantErr: assert.NoError,
		},
		{
			name:    "empty json",
			b:       []byte(`{}`),
			want:    &Unstructured{},
			wantErr: assert.NoError,
		},
		{
			name: "simple",
			b:    []byte(`{"foo":"bar"}`),
			want: &Unstructured{
				"foo": "bar",
			},
			wantErr: assert.NoError,
		},
		{
			name: "complex",
			b:    []byte(`{"foo":{"bar":123}}`),
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0, // float!
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(Unstructured) // nil receiver not allowed
			gotErr := got.UnmarshalJSON(tt.b)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, gotErr)
		})
		t.Run(tt.name, func(t *testing.T) {
			got := new(Unstructured)
			gotErr := json.Unmarshal(tt.b, got)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, gotErr)
		})
	}
}

func TestUnstructured_DeepCopy(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name string
		in   *Unstructured
	}{
		{
			name: "nil receiver",
			in:   nil,
		},
		{
			name: "nil map",
			in:   &nilMap,
		},
		{
			name: "empty",
			in:   &Unstructured{},
		},
		{
			name: "simple",
			in: &Unstructured{
				"foo": "bar",
			},
		},
		{
			name: "complex",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in.DeepCopy()
			assert.Equal(t, tt.in, got)
			if got != nil {
				assert.NotSame(t, tt.in, got)
			}
		})
	}
}

func TestUnstructured_DeepCopyInto(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name string
		in   *Unstructured
	}{
		// nil receiver not allowed
		{
			name: "nil map",
			in:   &nilMap,
		},
		{
			name: "empty",
			in:   &Unstructured{},
		},
		{
			name: "simple",
			in: &Unstructured{
				"foo": "bar",
			},
		},
		{
			name: "complex",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := new(Unstructured) // out not allowed to be nil
			tt.in.DeepCopyInto(out)
			assert.Equal(t, tt.in, out)
			if out != nil {
				assert.NotSame(t, tt.in, out)
			}
		})
	}
}

func TestUnstructured_Get(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name      string
		in        *Unstructured
		path      string
		want      interface{}
		wantFound bool
	}{
		{
			name:      "nil receiver",
			in:        nil,
			path:      "irrelevant",
			want:      nil,
			wantFound: false,
		},
		{
			name:      "nil map",
			in:        &nilMap,
			path:      "irrelevant",
			want:      nil,
			wantFound: false,
		},
		{
			name:      "empty",
			in:        &Unstructured{},
			path:      "irrelevant",
			want:      nil,
			wantFound: false,
		},
		{
			name: "simple found",
			in: &Unstructured{
				"foo": "bar",
			},
			path:      "foo",
			want:      "bar",
			wantFound: true,
		},
		{
			name: "simple not found",
			in: &Unstructured{
				"foo": "bar",
			},
			path:      "bar",
			want:      nil,
			wantFound: false,
		},
		{
			name: "complex found",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0,
				},
			},
			path:      "foo/bar",
			want:      123.0,
			wantFound: true,
		},
		{
			name: "complex not found",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0,
				},
			},
			path:      "foo/baz",
			want:      nil,
			wantFound: false,
		},
		{
			name: "complex not found 3",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123.0,
					},
				},
			},
			path:      "foo/baz",
			want:      nil,
			wantFound: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotFound := tt.in.Get(tt.path)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, gotFound)
		})
	}
}

func TestUnstructured_Put(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name string
		in   *Unstructured
		path string
		v    interface{}
		want *Unstructured
	}{
		{
			name: "nil receiver",
			in:   nil,
			path: "irrelevant",
			v:    "irrelevant",
			want: nil,
		},
		{
			name: "nil map",
			in:   &nilMap,
			path: "irrelevant",
			v:    "irrelevant",
			want: &nilMap,
		},
		{
			name: "empty",
			in:   &Unstructured{},
			path: "foo",
			v:    123,
			want: &Unstructured{
				"foo": 123,
			},
		},
		{
			name: "non empty",
			in:   &Unstructured{"foo": "bar"},
			path: "baz",
			v:    123,
			want: &Unstructured{
				"foo": "bar",
				"baz": 123,
			},
		},
		{
			name: "override",
			in:   &Unstructured{"foo": "bar"},
			path: "foo",
			v:    123,
			want: &Unstructured{
				"foo": 123,
			},
		},
		{
			name: "empty complex",
			in:   &Unstructured{},
			path: "foo/bar/qix",
			v:    123,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
		},
		{
			name: "non empty complex",
			in:   &Unstructured{"foo": "bar"},
			path: "baz/bar/qix",
			v:    123,
			want: &Unstructured{
				"foo": "bar",
				"baz": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
		},
		{
			name: "override complex 1",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
			path: "foo/bar/qix",
			v:    456,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 456,
					},
				},
			},
		},
		{
			name: "override complex 2",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
			path: "foo/bar/baz",
			v:    456,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
						"baz": 456,
					},
				},
			},
		},
		{
			name: "override complex 3",
			in:   &Unstructured{"foo": "bar"},
			path: "foo/bar/qix",
			v:    123,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.Put(tt.path, tt.v)
			assert.Equal(t, tt.want, tt.in)
		})
	}
}

func TestUnstructured_PutIfAbsent(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name string
		in   *Unstructured
		path string
		v    interface{}
		want *Unstructured
	}{
		{
			name: "nil receiver",
			in:   nil,
			path: "irrelevant",
			v:    "irrelevant",
			want: nil,
		},
		{
			name: "nil map",
			in:   &nilMap,
			path: "irrelevant",
			v:    "irrelevant",
			want: &nilMap,
		},
		{
			name: "empty",
			in:   &Unstructured{},
			path: "foo",
			v:    123,
			want: &Unstructured{
				"foo": 123,
			},
		},
		{
			name: "non empty",
			in:   &Unstructured{"foo": "bar"},
			path: "baz",
			v:    123,
			want: &Unstructured{
				"foo": "bar",
				"baz": 123,
			},
		},
		{
			name: "override",
			in:   &Unstructured{"foo": "bar"},
			path: "foo",
			v:    123,
			want: &Unstructured{
				"foo": "bar",
			},
		},
		{
			name: "empty complex",
			in:   &Unstructured{},
			path: "foo/bar/qix",
			v:    123,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
		},
		{
			name: "non empty complex",
			in:   &Unstructured{"foo": "bar"},
			path: "baz/bar/qix",
			v:    123,
			want: &Unstructured{
				"foo": "bar",
				"baz": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
		},
		{
			name: "override complex 1",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
			path: "foo/bar/qix",
			v:    456,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
		},
		{
			name: "override complex 2",
			in: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
					},
				},
			},
			path: "foo/bar/baz",
			v:    456,
			want: &Unstructured{
				"foo": map[string]interface{}{
					"bar": map[string]interface{}{
						"qix": 123,
						"baz": 456,
					},
				},
			},
		},
		{
			name: "override complex 3",
			in:   &Unstructured{"foo": "bar"},
			path: "foo/bar/qix",
			v:    123,
			want: &Unstructured{
				"foo": "bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.PutIfAbsent(tt.path, tt.v)
			assert.Equal(t, tt.want, tt.in)
		})
	}
}

func TestUnstructured_PutAll(t *testing.T) {
	nilMap := Unstructured(nil)
	tests := []struct {
		name string
		in   *Unstructured
		m    map[string]interface{}
		want *Unstructured
	}{
		{
			name: "nil receiver nil map",
			in:   nil,
			m:    nil,
			want: nil,
		},
		{
			name: "nil receiver empty map",
			in:   nil,
			m:    map[string]interface{}{},
			want: nil,
		},
		{
			name: "nil receiver non empty map",
			in:   nil,
			m:    map[string]interface{}{"foo": "bar"},
			want: nil,
		},
		{
			name: "nil map nil map",
			in:   &nilMap,
			m:    nil,
			want: &Unstructured{},
		},
		{
			name: "nil map empty map",
			in:   &nilMap,
			m:    map[string]interface{}{},
			want: &Unstructured{},
		},
		{
			name: "nil map non empty map",
			in:   &nilMap,
			m:    map[string]interface{}{"foo": "bar"},
			want: &Unstructured{"foo": "bar"},
		},
		{
			name: "empty receiver nil map",
			in:   &Unstructured{},
			m:    nil,
			want: &Unstructured{},
		},
		{
			name: "empty receiver empty map",
			in:   &Unstructured{},
			m:    map[string]interface{}{},
			want: &Unstructured{},
		},
		{
			name: "empty receiver non empty map",
			in:   &Unstructured{},
			m:    map[string]interface{}{"foo": "bar"},
			want: &Unstructured{"foo": "bar"},
		},
		{
			name: "non empty receiver nil map",
			in:   &Unstructured{"foo": "bar"},
			m:    nil,
			want: &Unstructured{"foo": "bar"},
		},
		{
			name: "non empty receiver empty map",
			in:   &Unstructured{"foo": "bar"},
			m:    map[string]interface{}{},
			want: &Unstructured{"foo": "bar"},
		},
		{
			name: "non empty receiver non empty map",
			in:   &Unstructured{"foo": "bar"},
			m:    map[string]interface{}{"baz": "qix"},
			want: &Unstructured{"foo": "bar", "baz": "qix"},
		},
		{
			name: "non empty receiver non empty map with override",
			in:   &Unstructured{"foo": "bar"},
			m:    map[string]interface{}{"foo": "qix"},
			want: &Unstructured{"foo": "qix"},
		},
		{
			name: "non empty receiver non empty map with override nested",
			in:   &Unstructured{"foo": map[string]interface{}{"bar": "qix"}},
			m:    map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			want: &Unstructured{"foo": map[string]interface{}{"bar": "baz"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.in.PutAll(tt.m)
			assert.Equal(t, tt.want, tt.in)
		})
	}
}
