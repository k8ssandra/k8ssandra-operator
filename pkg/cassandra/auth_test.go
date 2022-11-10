package cassandra

import (
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestApplyAuthSettings(t *testing.T) {
	tests := []struct {
		name        string
		authEnabled bool
		input       k8ssandraapi.CassandraConfig
		want        k8ssandraapi.CassandraConfig
	}{
		{
			"auth enabled",
			true,
			k8ssandraapi.CassandraConfig{},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"authenticator": "PasswordAuthenticator",
					"authorizer":    "CassandraAuthorizer",
					"role_manager":  "CassandraRoleManager",
				},
			},
		},
		{
			"auth enabled custom values",
			true,
			k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"authenticator": "MyAuthenticator",
					"authorizer":    "MyAuthorizer",
					"role_manager":  "MyRoleManager",
				},
			},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"authenticator": "MyAuthenticator",
					"authorizer":    "MyAuthorizer",
					"role_manager":  "MyRoleManager",
				},
			},
		},
		{
			"auth disabled",
			false,
			k8ssandraapi.CassandraConfig{},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"authenticator": "AllowAllAuthenticator",
					"authorizer":    "AllowAllAuthorizer",
					"role_manager":  "CassandraRoleManager",
				},
			},
		},
		{
			"auth disabled custom values",
			false,
			k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"authenticator": "MyAuthenticator",
					"authorizer":    "MyAuthorizer",
					"role_manager":  "MyRoleManager",
				},
			},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{
					"authenticator": "MyAuthenticator",
					"authorizer":    "MyAuthorizer",
					"role_manager":  "MyRoleManager",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ApplyAuthSettings(tt.input, tt.authEnabled, k8ssandraapi.ServerDistributionCassandra)
			assert.Equal(t, tt.want, actual)
		})
	}
}
