package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMergeMapNested(t *testing.T) {
	tests := []struct {
		name           string
		allowOverwrite bool
		sources        []map[string]interface{}
		want           map[string]interface{}
		wantErr        assert.ErrorAssertionFunc
	}{
		{
			"no source",
			false,
			[]map[string]interface{}{},
			map[string]interface{}{},
			assert.NoError,
		},
		{
			"nil sources",
			false,
			[]map[string]interface{}{nil, nil},
			map[string]interface{}{},
			assert.NoError,
		},
		{
			"empty sources",
			false,
			[]map[string]interface{}{{}, {}},
			map[string]interface{}{},
			assert.NoError,
		},
		{
			"single source",
			false,
			[]map[string]interface{}{{"foo": 42}},
			map[string]interface{}{"foo": 42},
			assert.NoError,
		},
		{
			"simple merge",
			false,
			[]map[string]interface{}{{"foo": 42}, {"bar": 42}},
			map[string]interface{}{"foo": 42, "bar": 42},
			assert.NoError,
		},
		{
			"simple merge overlapping no overwrite",
			false,
			[]map[string]interface{}{{"foo": 42}, {"foo": "bar"}},
			nil,
			assert.Error,
		},
		{
			"simple merge overlapping overwrite",
			true,
			[]map[string]interface{}{{"foo": 42}, {"foo": "bar"}},
			map[string]interface{}{"foo": "bar"},
			assert.NoError,
		},
		{
			"nested merge non-overlapping",
			false,
			[]map[string]interface{}{{"foo": map[string]interface{}{"a": 123, "b": 456}}, {"bar": map[string]interface{}{"c": 123, "d": 456}}},
			map[string]interface{}{"foo": map[string]interface{}{"a": 123, "b": 456}, "bar": map[string]interface{}{"c": 123, "d": 456}},
			assert.NoError,
		},
		{
			"nested merge overlapping no overwrite",
			false,
			[]map[string]interface{}{{"foo": map[string]interface{}{"a": 123, "b": 456}}, {"foo": map[string]interface{}{"b": 123, "c": 456}}},
			nil,
			assert.Error,
		},
		{
			"nested merge overlapping overwrite",
			true,
			[]map[string]interface{}{{"foo": map[string]interface{}{"a": 123, "b": 456}}, {"foo": map[string]interface{}{"b": 123, "c": 456}}},
			map[string]interface{}{"foo": map[string]interface{}{"a": 123, "b": 123, "c": 456}},
			assert.NoError,
		},
		{
			"nested merge overlapping map vs non-map no overwrite",
			false,
			[]map[string]interface{}{{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}}, {"foo": map[string]interface{}{"bar": 42}}},
			nil,
			assert.Error,
		},
		{
			"nested merge overlapping non-map vs map no overwrite",
			false,
			[]map[string]interface{}{{"foo": map[string]interface{}{"bar": 42}}, {"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}}},
			nil,
			assert.Error,
		},
		{
			"nested merge overlapping map vs non-map overwrite",
			true,
			[]map[string]interface{}{{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}}, {"foo": map[string]interface{}{"bar": 42}}},
			map[string]interface{}{"foo": map[string]interface{}{"bar": 42}},
			assert.NoError,
		},
		{
			"nested merge overlapping non-map vs map overwrite",
			true,
			[]map[string]interface{}{{"foo": map[string]interface{}{"bar": 42}}, {"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}}},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}},
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := MergeMapNested(tt.allowOverwrite, tt.sources...)
			assert.Equal(t, tt.want, got)
			tt.wantErr(t, gotErr)
		})
	}
}

func TestGetMapNested(t *testing.T) {
	tests := []struct {
		name      string
		m         map[string]interface{}
		keys      []string
		want      interface{}
		wantFound bool
	}{
		{
			"simple found",
			map[string]interface{}{"foo": 42},
			[]string{"foo"},
			42,
			true,
		},
		{
			"simple not found",
			map[string]interface{}{"foo": 42},
			[]string{"bar"},
			nil,
			false,
		},
		{
			"nested found",
			map[string]interface{}{"foo": map[string]interface{}{"bar": 42}},
			[]string{"foo", "bar"},
			42,
			true,
		},
		{
			"nested not found",
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}},
			[]string{"bar", "qix"},
			nil,
			false,
		},
		{
			"nested wrong type",
			map[string]interface{}{"foo": 42},
			[]string{"foo", "bar"},
			nil,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotFound := GetMapNested(tt.m, tt.keys[0], tt.keys[1:]...)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantFound, gotFound)
		})
	}
}

func TestPutMapNested(t *testing.T) {
	tests := []struct {
		name           string
		allowOverwrite bool
		in             map[string]interface{}
		keys           []string
		want           map[string]interface{}
		wantErr        assert.ErrorAssertionFunc
	}{
		{
			"simple",
			false,
			map[string]interface{}{},
			[]string{"foo"},
			map[string]interface{}{"foo": 42},
			assert.NoError,
		},
		{
			"nested",
			false,
			map[string]interface{}{},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}},
			assert.NoError,
		},
		{
			"existing keys no overlap",
			false,
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"irrelevant": 41}}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"irrelevant": 41, "qix": 42}}},
			assert.NoError,
		},
		{
			"existing keys overlap no overwrite",
			false,
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 41}}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 41}}},
			assert.Error,
		},
		{
			"existing keys overlap overwrite",
			true,
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 41}}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}},
			assert.NoError,
		},
		{
			"intermediary key wrong type overlap no overwrite",
			false,
			map[string]interface{}{"foo": []int{41, 42}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": []int{41, 42}},
			assert.Error,
		},
		{
			"intermediary key wrong type overlap overwrite",
			true,
			map[string]interface{}{"foo": []int{41, 42}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}},
			assert.NoError,
		},
		{
			"last key wrong type overlap no overwrite",
			false,
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": "41"}}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": "41"}}},
			assert.Error,
		},
		{
			"last key wrong type overlap overwrite",
			true,
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": "41"}}},
			[]string{"foo", "bar", "qix"},
			map[string]interface{}{"foo": map[string]interface{}{"bar": map[string]interface{}{"qix": 42}}},
			assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := PutMapNested(tt.allowOverwrite, tt.in, 42, tt.keys[0], tt.keys[1:]...)
			assert.Equal(t, tt.want, tt.in)
			tt.wantErr(t, gotErr)
		})
	}
}
