package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFirstNonEmptyString(t *testing.T) {
	tests := []struct {
		name string
		strs []string
		want string
	}{
		{
			name: "empty",
			strs: []string{},
			want: "",
		},
		{
			name: "one empty",
			strs: []string{""},
			want: "",
		},
		{
			name: "one non-empty",
			strs: []string{"foo"},
			want: "foo",
		},
		{
			name: "two non-empty",
			strs: []string{"foo", "bar"},
			want: "foo",
		},
		{
			name: "first non-empty, second empty",
			strs: []string{"foo", ""},
			want: "foo",
		},
		{
			name: "first empty, second non-empty",
			strs: []string{"", "bar"},
			want: "bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FirstNonEmptyString(tt.strs...)
			assert.Equal(t, tt.want, got)
		})
	}
}
