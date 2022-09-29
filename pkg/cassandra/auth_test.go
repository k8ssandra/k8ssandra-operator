package cassandra

import (
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestApplyAuthSettings(t *testing.T) {
	tests := []struct {
		name        string
		authEnabled bool
		input       unstructured.Unstructured
		want        unstructured.Unstructured
	}{
		{
			"auth enabled",
			true,
			unstructured.Unstructured{},
			unstructured.Unstructured{
				"authenticator": "PasswordAuthenticator",
				"authorizer":    "CassandraAuthorizer",
				"role_manager":  "CassandraRoleManager",
			},
		},
		{
			"auth enabled custom values",
			true,
			unstructured.Unstructured{
				"authenticator": "MyAuthenticator",
				"authorizer":    "MyAuthorizer",
				"role_manager":  "MyRoleManager",
			},
			unstructured.Unstructured{
				"authenticator": "MyAuthenticator",
				"authorizer":    "MyAuthorizer",
				"role_manager":  "MyRoleManager",
			},
		},
		{
			"auth disabled",
			false,
			unstructured.Unstructured{},
			unstructured.Unstructured{
				"authenticator": "AllowAllAuthenticator",
				"authorizer":    "AllowAllAuthorizer",
				"role_manager":  "CassandraRoleManager",
			},
		},
		{
			"auth disabled custom values",
			false,
			unstructured.Unstructured{
				"authenticator": "MyAuthenticator",
				"authorizer":    "MyAuthorizer",
				"role_manager":  "MyRoleManager",
			},
			unstructured.Unstructured{
				"authenticator": "MyAuthenticator",
				"authorizer":    "MyAuthorizer",
				"role_manager":  "MyRoleManager",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyAuthSettings(tt.input, tt.authEnabled)
			assert.Equal(t, tt.want, tt.input)
		})
	}
}
