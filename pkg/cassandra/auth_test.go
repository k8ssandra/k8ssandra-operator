package cassandra

import (
	"testing"

	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/unstructured"
	"github.com/stretchr/testify/assert"
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

func TestApplyAuth(t *testing.T) {
	tests := []struct {
		name                  string
		authEnabled           bool
		useExternalSecrets    bool
		reaperRequiresJmx     bool
		initialConfig         *DatacenterConfig
		expectedJvmOptions    []string
		expectedAuthenticator string
		expectedAuthorizer    string
		expectedRoleManager   string
	}{
		{
			name:               "auth disabled, no external secrets, no reaper JMX",
			authEnabled:        false,
			useExternalSecrets: false,
			reaperRequiresJmx:  false,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{},
			expectedAuthenticator: "AllowAllAuthenticator",
			expectedAuthorizer:    "AllowAllAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth disabled, no external secrets, reaper JMX enabled",
			authEnabled:        false,
			useExternalSecrets: false,
			reaperRequiresJmx:  true,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{"-Dcom.sun.management.jmxremote.authenticate=false"},
			expectedAuthenticator: "AllowAllAuthenticator",
			expectedAuthorizer:    "AllowAllAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth disabled, external secrets, no reaper JMX",
			authEnabled:        false,
			useExternalSecrets: true,
			reaperRequiresJmx:  false,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{},
			expectedAuthenticator: "AllowAllAuthenticator",
			expectedAuthorizer:    "AllowAllAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth disabled, external secrets, reaper JMX enabled",
			authEnabled:        false,
			useExternalSecrets: true,
			reaperRequiresJmx:  true,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{"-Dcom.sun.management.jmxremote.authenticate=false"},
			expectedAuthenticator: "AllowAllAuthenticator",
			expectedAuthorizer:    "AllowAllAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth enabled, no external secrets, no reaper JMX",
			authEnabled:        true,
			useExternalSecrets: false,
			reaperRequiresJmx:  false,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{},
			expectedAuthenticator: "PasswordAuthenticator",
			expectedAuthorizer:    "CassandraAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth enabled, no external secrets, reaper JMX enabled",
			authEnabled:        true,
			useExternalSecrets: false,
			reaperRequiresJmx:  true,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions: []string{
				"-Dcassandra.jmx.authorizer=org.apache.cassandra.auth.jmx.AuthorizationProxy",
				"-Djava.security.auth.login.config=$CASSANDRA_HOME/conf/cassandra-jaas.config",
				"-Dcassandra.jmx.remote.login.config=CassandraLogin",
				"-Dcom.sun.management.jmxremote.authenticate=true",
			},
			expectedAuthenticator: "PasswordAuthenticator",
			expectedAuthorizer:    "CassandraAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth enabled, external secrets, no reaper JMX",
			authEnabled:        true,
			useExternalSecrets: true,
			reaperRequiresJmx:  false,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{},
			expectedAuthenticator: "PasswordAuthenticator",
			expectedAuthorizer:    "CassandraAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "auth enabled, external secrets, reaper JMX enabled",
			authEnabled:        true,
			useExternalSecrets: true,
			reaperRequiresJmx:  true,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionCassandra,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{"-Dcom.sun.management.jmxremote.authenticate=true"},
			expectedAuthenticator: "PasswordAuthenticator",
			expectedAuthorizer:    "CassandraAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
		{
			name:               "DSE auth enabled, no external secrets, reaper JMX enabled",
			authEnabled:        true,
			useExternalSecrets: false,
			reaperRequiresJmx:  true,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionDse,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions: []string{
				"-Dcassandra.jmx.authorizer=org.apache.cassandra.auth.jmx.AuthorizationProxy",
				"-Djava.security.auth.login.config=$CASSANDRA_HOME/conf/cassandra-jaas.config",
				"-Dcassandra.jmx.remote.login.config=CassandraLogin",
				"-Dcom.sun.management.jmxremote.authenticate=true",
			},
			expectedAuthenticator: "com.datastax.bdp.cassandra.auth.DseAuthenticator",
			expectedAuthorizer:    "com.datastax.bdp.cassandra.auth.DseAuthorizer",
			expectedRoleManager:   "com.datastax.bdp.cassandra.auth.DseRoleManager",
		},
		{
			name:               "DSE auth disabled, no external secrets, no reaper JMX",
			authEnabled:        false,
			useExternalSecrets: false,
			reaperRequiresJmx:  false,
			initialConfig: &DatacenterConfig{
				ServerType: k8ssandraapi.ServerDistributionDse,
				CassandraConfig: k8ssandraapi.CassandraConfig{
					CassandraYaml: unstructured.Unstructured{},
					JvmOptions: k8ssandraapi.JvmOptions{
						AdditionalOptions: []string{},
					},
				},
			},
			expectedJvmOptions:    []string{},
			expectedAuthenticator: "AllowAllAuthenticator",
			expectedAuthorizer:    "AllowAllAuthorizer",
			expectedRoleManager:   "CassandraRoleManager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of the initial config to avoid modifying the original
			dcConfig := *tt.initialConfig
			dcConfig.CassandraConfig = k8ssandraapi.CassandraConfig{
				CassandraYaml: unstructured.Unstructured{},
				DseYaml:       unstructured.Unstructured{},
				JvmOptions: k8ssandraapi.JvmOptions{
					AdditionalOptions: []string{},
				},
			}

			// Apply the auth settings
			ApplyAuth(&dcConfig, tt.authEnabled, tt.useExternalSecrets, tt.reaperRequiresJmx)

			// Verify JVM options
			assert.ElementsMatch(t, tt.expectedJvmOptions, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions,
				"JVM options should match expected values")

			// Verify authenticator, authorizer, and role manager settings
			assert.Equal(t, tt.expectedAuthenticator, dcConfig.CassandraConfig.CassandraYaml["authenticator"],
				"Authenticator should be set correctly")
			assert.Equal(t, tt.expectedAuthorizer, dcConfig.CassandraConfig.CassandraYaml["authorizer"],
				"Authorizer should be set correctly")
			assert.Equal(t, tt.expectedRoleManager, dcConfig.CassandraConfig.CassandraYaml["role_manager"],
				"Role manager should be set correctly")

			// Verify DSE-specific settings if applicable
			if tt.initialConfig.ServerType == k8ssandraapi.ServerDistributionDse && tt.authEnabled {
				authOptions, exists := dcConfig.CassandraConfig.DseYaml["authentication_options"]
				assert.True(t, exists, "DSE authentication_options should exist")
				if authOptionsMap, ok := authOptions.(map[string]interface{}); ok {
					assert.Equal(t, "true", authOptionsMap["enabled"], "DSE authentication should be enabled")
				}

				authzOptions, exists := dcConfig.CassandraConfig.DseYaml["authorization_options"]
				assert.True(t, exists, "DSE authorization_options should exist")
				if authzOptionsMap, ok := authzOptions.(map[string]interface{}); ok {
					assert.Equal(t, "true", authzOptionsMap["enabled"], "DSE authorization should be enabled")
				}

				roleOptions, exists := dcConfig.CassandraConfig.DseYaml["role_management_options"]
				assert.True(t, exists, "DSE role_management_options should exist")
				if roleOptionsMap, ok := roleOptions.(map[string]interface{}); ok {
					assert.Equal(t, "internal", roleOptionsMap["mode"], "DSE role management should be set to internal")
				}
			}
		})
	}
}

func TestApplyAuth_WithExistingOptions(t *testing.T) {
	// Test that existing JVM options are preserved and new ones are added
	dcConfig := &DatacenterConfig{
		ServerType: k8ssandraapi.ServerDistributionCassandra,
		CassandraConfig: k8ssandraapi.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{},
			JvmOptions: k8ssandraapi.JvmOptions{
				AdditionalOptions: []string{"-Xmx2g", "-Xms2g"},
			},
		},
	}

	ApplyAuth(dcConfig, true, false, true)

	// Verify that existing options are preserved
	assert.Contains(t, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, "-Xmx2g")
	assert.Contains(t, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, "-Xms2g")

	// Verify that new auth-related options are added
	assert.Contains(t, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, "-Dcom.sun.management.jmxremote.authenticate=true")
	assert.Contains(t, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, "-Dcassandra.jmx.remote.login.config=CassandraLogin")
	assert.Contains(t, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, "-Djava.security.auth.login.config=$CASSANDRA_HOME/conf/cassandra-jaas.config")
	assert.Contains(t, dcConfig.CassandraConfig.JvmOptions.AdditionalOptions, "-Dcassandra.jmx.authorizer=org.apache.cassandra.auth.jmx.AuthorizationProxy")
}

func TestApplyAuth_NoDuplicateOptions(t *testing.T) {
	// Test that duplicate options are not added
	dcConfig := &DatacenterConfig{
		ServerType: k8ssandraapi.ServerDistributionCassandra,
		CassandraConfig: k8ssandraapi.CassandraConfig{
			CassandraYaml: unstructured.Unstructured{},
			JvmOptions: k8ssandraapi.JvmOptions{
				AdditionalOptions: []string{"-Dcom.sun.management.jmxremote.authenticate=true"},
			},
		},
	}

	ApplyAuth(dcConfig, true, false, true)

	// Count occurrences of the authenticate option
	count := 0
	for _, option := range dcConfig.CassandraConfig.JvmOptions.AdditionalOptions {
		if option == "-Dcom.sun.management.jmxremote.authenticate=true" {
			count++
		}
	}

	assert.Equal(t, 1, count, "Should not add duplicate JVM options")
}
