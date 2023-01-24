package yq

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func TestEval(t *testing.T) {
	type args struct {
		expression string
		options    Options
		files      []string
	}
	type test struct {
		name    string
		args    args
		want    string
		wantErr string
		assert  func()
	}
	tests := []test{
		{
			name: "count dcs",
			args: args{
				expression: ".spec.cassandra.datacenters.[] as $item ireduce (0; . +1)",
				files:      []string{"../testdata/fixtures/multi-dc/k8ssandra.yaml"},
			},
			want: "2",
		},
		{
			name: "count dcs many files",
			args: args{
				expression: ".spec.cassandra.datacenters.[] as $item ireduce (0; . +1)",
				files: []string{
					"../testdata/fixtures/multi-dc/k8ssandra.yaml",
					"../testdata/fixtures/single-dc/k8ssandra.yaml",
				},
			},
			want: "2\n1",
		},
		{
			name: "count dcs many files eval all",
			args: args{
				expression: ".spec.cassandra.datacenters.[] as $item ireduce (0; . +1)",
				options:    Options{All: true},
				files: []string{
					"../testdata/fixtures/multi-dc/k8ssandra.yaml",
					"../testdata/fixtures/single-dc/k8ssandra.yaml",
				},
			},
			want: "3",
		},
		{
			name: "empty line",
			args: args{
				expression: ".spec.cassandra.datacenters.[] | { .metadata.name : .k8sContext } | to_entries | .[] | .value // \"\"",
				files: []string{
					"../testdata/fixtures/multi-dc-medusa/k8ssandra.yaml",
				},
			},
			want: "\nkind-k8ssandra-1",
		},
		{
			name: "non existent file",
			args: args{
				expression: ".spec.cassandra.datacenters.[] as $item ireduce (0; . +1)",
				files:      []string{"non-existent.yaml"},
			},
			wantErr: "eval failed: Error: open non-existent.yaml: no such file or directory (exit status 1)",
		},
		func() test {
			file, _ := os.CreateTemp("", "test-*.yaml")
			_ = os.WriteFile(file.Name(), []byte("foo: bar"), 0644)
			return test{
				name: "in place",
				args: args{
					expression: ".foo |= \"qix\"",
					options:    Options{InPlace: true},
					files:      []string{file.Name()},
				},
				assert: func() {
					yaml, _ := os.ReadFile(file.Name())
					assert.Equal(t, "foo: qix", strings.TrimSpace(string(yaml)))
				},
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.args.expression, tt.args.options, tt.args.files...)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
			assert.Equal(t, tt.want, got)
			if tt.assert != nil {
				tt.assert()
			}
		})
	}
}
