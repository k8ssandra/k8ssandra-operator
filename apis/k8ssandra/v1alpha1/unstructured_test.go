package v1alpha1

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUnstructured_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		u       Unstructured
		want    []byte
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "nil",
			u:       nil,
			want:    []byte(`null`),
			wantErr: assert.NoError,
		},
		{
			name:    "empty",
			u:       Unstructured{},
			want:    []byte(`{}`),
			wantErr: assert.NoError,
		},
		{
			name: "simple",
			u: Unstructured{
				"foo": "bar",
			},
			want:    []byte(`{"foo":"bar"}`),
			wantErr: assert.NoError,
		},
		{
			name: "complex",
			u: Unstructured{
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
			got, err := json.Marshal(tt.u)
			if !tt.wantErr(t, err, fmt.Sprintf("MarshalJSON()")) {
				return
			}
			assert.Equalf(t, tt.want, got, "MarshalJSON()")
		})
	}
}

func TestUnstructured_UnmarshalJSON(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		args    args
		want    Unstructured
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "null",
			args: args{
				b: []byte(`null`),
			},
			want:    Unstructured(nil),
			wantErr: assert.NoError,
		},
		{
			name: "empty",
			args: args{
				b: []byte(`{}`),
			},
			want:    Unstructured{},
			wantErr: assert.NoError,
		},
		{
			name: "simple",
			args: args{
				b: []byte(`{"foo":"bar"}`),
			},
			want: Unstructured{
				"foo": "bar",
			},
			wantErr: assert.NoError,
		},
		{
			name: "complex",
			args: args{
				b: []byte(`{"foo":{"bar":123}}`),
			},
			want: Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0, // float!
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Unstructured{}
			if !tt.wantErr(t, json.Unmarshal(tt.args.b, &v), fmt.Sprintf("UnmarshalJSON()")) {
				return
			}
			assert.Equalf(t, tt.want, v, fmt.Sprintf("UnmarshalJSON(%v)", tt.args.b))
		})
	}
}

func TestUnstructured_DeepCopy(t *testing.T) {
	tests := []struct {
		name string
		u    Unstructured
	}{
		{
			name: "nil",
			u:    nil,
		},
		{
			name: "empty",
			u:    Unstructured{},
		},
		{
			name: "simple",
			u: Unstructured{
				"foo": "bar",
			},
		},
		{
			name: "complex",
			u: Unstructured{
				"foo": map[string]interface{}{
					"bar": 123.0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.u, *tt.u.DeepCopy(), "DeepCopy()")
			assert.NotSamef(t, tt.u, *tt.u.DeepCopy(), "DeepCopy()")
		})
	}
}
