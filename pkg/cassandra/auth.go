package cassandra

import (
	"fmt"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// ApplyAuth modifies the dc config depending on whether auth is enabled in the cluster or not.
func ApplyAuth(dcConfig *DatacenterConfig, authEnabled bool, useExternalSecrets bool, enableJmxAuth bool) {

	dcConfig.CassandraConfig = ApplyAuthSettings(dcConfig.CassandraConfig, authEnabled, dcConfig.ServerType)

	// By default, the Cassandra process will be started with LOCAL_JMX=yes, see cassandra-env.sh. This means that the
	// Cassandra process will only be accessible with JMX from localhost. This is the safest and preferred setup: you
	// still can use JMX by SSH'ing into the Cassandra pod, for example to run nodetool. But some components need remote
	// JMX access (Reaper, metrics, etc.). Such components and their controllers are responsible for setting
	// LOCAL_JMX=no whenever appropriate, to enable remote JMX access.
	// However, authentication will get in the way, even if it's an orthogonal concern. Indeed, with LOCAL_JMX=yes
	// cassandra-env.sh will infer that no JMX authentication should be used
	// (com.sun.management.jmxremote.authenticate=false), whereas with LOCAL_JMX=no it will infer that authentication is
	// required (com.sun.management.jmxremote.authenticate=true). We need to change that here and enable/disable
	// authentication based on what the user specified, not what the script infers.
	jmxAuthenticateOpt := fmt.Sprintf("-Dcom.sun.management.jmxremote.authenticate=%v", authEnabled)
	addOptionIfMissing(dcConfig, jmxAuthenticateOpt)

	// Use Cassandra internals for JMX authentication and authorization. This allows JMX clients to connect with the
	// superuser secret.
	if authEnabled && !useExternalSecrets && enableJmxAuth {
		addOptionIfMissing(dcConfig, "-Dcassandra.jmx.remote.login.config=CassandraLogin")
		addOptionIfMissing(dcConfig, "-Djava.security.auth.login.config=$CASSANDRA_HOME/conf/cassandra-jaas.config")
		addOptionIfMissing(dcConfig, "-Dcassandra.jmx.authorizer=org.apache.cassandra.auth.jmx.AuthorizationProxy")
	}
}

// ApplyAuthSettings modifies the given config and applies defaults for authenticator, authorizer and role manager,
// depending on whether auth is enabled or not, and only if these settings are empty in the input config.
func ApplyAuthSettings(config api.CassandraConfig, authEnabled bool, serverType api.ServerDistribution) api.CassandraConfig {
	if authEnabled {
		if serverType == api.ServerDistributionDse {
			config.CassandraYaml.PutIfAbsent("authenticator", "com.datastax.bdp.cassandra.auth.DseAuthenticator")
			config.CassandraYaml.PutIfAbsent("authorizer", "com.datastax.bdp.cassandra.auth.DseAuthorizer")
			config.CassandraYaml.PutIfAbsent("role_manager", "com.datastax.bdp.cassandra.auth.DseRoleManager")

			config.DseYaml.PutIfAbsent("authentication_options/enabled", "true")
			config.DseYaml.PutIfAbsent("authorization_options/enabled", "true")
			config.DseYaml.PutIfAbsent("role_management_options/mode", "internal")
		} else {
			config.CassandraYaml.PutIfAbsent("authenticator", "PasswordAuthenticator")
			config.CassandraYaml.PutIfAbsent("authorizer", "CassandraAuthorizer")
		}
	} else {
		config.CassandraYaml.PutIfAbsent("authenticator", "AllowAllAuthenticator")
		config.CassandraYaml.PutIfAbsent("authorizer", "AllowAllAuthorizer")
	}
	config.CassandraYaml.PutIfAbsent("role_manager", "CassandraRoleManager")
	return config
}

// If auth is enabled in this cluster, we need to allow components to access the cluster through CQL. This is done by
// declaring a Cassandra user whose credentials are pulled from CassandraUserSecretRef.
func AddCqlUser(cassandraUserSecretRef corev1.LocalObjectReference, dcConfig *DatacenterConfig, cassandraUserSecretName string) {
	if cassandraUserSecretRef.Name == "" {
		cassandraUserSecretRef.Name = cassandraUserSecretName
	}
	dcConfig.Users = append(dcConfig.Users, cassdcapi.CassandraUser{
		SecretName: cassandraUserSecretRef.Name,
		Superuser:  true,
	})
}
