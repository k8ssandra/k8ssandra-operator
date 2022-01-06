package cassandra

import (
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
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
				CassandraYaml: k8ssandraapi.CassandraYaml{
					Authenticator: pointer.String("PasswordAuthenticator"),
					Authorizer:    pointer.String("CassandraAuthorizer"),
					RoleManager:   pointer.String("CassandraRoleManager"),
				},
			},
		},
		{
			"auth enabled custom values",
			true,
			k8ssandraapi.CassandraConfig{
				CassandraYaml: k8ssandraapi.CassandraYaml{
					Authenticator: pointer.String("MyAuthenticator"),
					Authorizer:    pointer.String("MyAuthorizer"),
					RoleManager:   pointer.String("MyRoleManager"),
				},
			},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: k8ssandraapi.CassandraYaml{
					Authenticator: pointer.String("MyAuthenticator"),
					Authorizer:    pointer.String("MyAuthorizer"),
					RoleManager:   pointer.String("MyRoleManager"),
				},
			},
		},
		{
			"auth disabled",
			false,
			k8ssandraapi.CassandraConfig{},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: k8ssandraapi.CassandraYaml{
					Authenticator: pointer.String("AllowAllAuthenticator"),
					Authorizer:    pointer.String("AllowAllAuthorizer"),
					RoleManager:   pointer.String("CassandraRoleManager"),
				},
			},
		},
		{
			"auth disabled custom values",
			false,
			k8ssandraapi.CassandraConfig{
				CassandraYaml: k8ssandraapi.CassandraYaml{
					Authenticator: pointer.String("MyAuthenticator"),
					Authorizer:    pointer.String("MyAuthorizer"),
					RoleManager:   pointer.String("MyRoleManager"),
				},
			},
			k8ssandraapi.CassandraConfig{
				CassandraYaml: k8ssandraapi.CassandraYaml{
					Authenticator: pointer.String("MyAuthenticator"),
					Authorizer:    pointer.String("MyAuthorizer"),
					RoleManager:   pointer.String("MyRoleManager"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ApplyAuthSettings(tt.input, tt.authEnabled)
			assert.Equal(t, tt.want, actual)
		})
	}
}
