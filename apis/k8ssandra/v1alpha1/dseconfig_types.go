/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import "k8s.io/apimachinery/pkg/api/resource"

type DseYaml struct {

	// Configures DseAuthenticator to authenticate users when the authenticator option in
	// cassandra.yaml is set to com.datastax.bdp.cassandra.auth.DseAuthenticator. Authenticators
	// other than DseAuthenticator are not supported.
	// +optional
	AuthenticationOptions *AuthenticationOptions `json:"authentication_options,omitempty" cass-config:"dse:*:authentication_options;recurse"`

	// Configures the DSE Role Manager. To enable role manager, set:
	// - authorization_options enabled to true
	// - role_manager in cassandra.yaml to com.datastax.bdp.cassandra.auth.DseRoleManager
	// When scheme_permissions is enabled, all roles must have permission to execute on the
	// authentication scheme.
	// +optional
	RoleManagementOptions *RoleManagementOptions `json:"role_management_options,omitempty" cass-config:"dse:*:role_management_options;recurse"`

	// Configures the DSE Authorizer to authorize users when the authorization option in
	// cassandra.yaml is set to com.datastax.bdp.cassandra.auth.DseAuthorizer.
	// +optional
	AuthorizationOptions *AuthorizationOptions `json:"authorization_options,omitempty" cass-config:"dse:*:authorization_options;recurse"`

	// Configures security for a DataStax Enterprise cluster using Kerberos when the authenticator
	// option in cassandra.yaml is set to com.datastax.bdp.cassandra.auth.DseAuthenticator.
	// +optional
	KerberosOptions *KerberosOptions `json:"kerberos_options,omitempty" cass-config:"dse:*:kerberos_options;recurse"`

	// Configures LDAP security when the authenticator option in cassandra.yaml is set to
	// com.datastax.bdp.cassandra.auth.DseAuthenticator.
	// +optional
	LdapOptions *LdapOptions `json:"ldap_options,omitempty" cass-config:"dse:*:ldap_options;recurse"`

	// Sets the encryption settings for system resources that might contain sensitive information,
	// including the system.batchlog and system.paxos tables, hint files, and the database commit
	// log.
	// +optional
	SystemInfoEncryptionOptions *SystemInfoEncryptionOptions `json:"system_info_encryption,omitempty" cass-config:"dse:*:system_info_encryption;recurse"`

	// Settings for using encrypted passwords in sensitive configuration file properties.
	// +optional
	ConfigurationEncryptionOptions *ConfigurationEncryptionOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Options for KMIP encryption keys and communication between the DataStax Enterprise node and
	// the KMIP key server or key servers. Enables DataStax Enterprise encryption features to use
	// encryption keys that are stored on a server that is not running DataStax Enterprise.
	// +optional
	KmipEncryptionOptions *KmipEncryptionOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Tunes encryption of search indexes.
	// +optional
	SolrEncryptionOptions *SolrEncryptionOptions `json:"solr_encryption_options,omitempty" cass-config:"dse:*:solr_encryption_options;recurse"`

	// To use DSE In-Memory, specify how much system memory to use for all in-memory tables by
	// fraction or size.
	// +optional
	DseInMemoryOptions *DseInMemoryOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Node health options are always enabled. Node health is a score-based representation of how
	// healthy a node is to handle search queries.
	// +optional
	NodeHealthOptions *NodeHealthOptions `json:"node_health_options" cass-config:"dse:*:node_health_options;recurse"`

	// Enables node health as a consideration for replication selection for distributed DSE Search
	// queries. Health-based routing enables a trade-off between index consistency and query
	// throughput.
	// - true: Consider node health when multiple candidates exist for a particular token range.
	// - false: Ignore node health for replication selection. When the primary concern is
	//   performance, do not enable health-based routing.
	// Default: true
	// +optional
	EnableHealthBasedRouting *bool `json:"enable_health_based_routing,omitempty" cass-config:"dse:*:enable_health_based_routing"`

	// Lease holder statistics help monitor the lease subsystem for automatic management of Job
	// Tracker and Spark Master nodes.
	// +optional
	LeaseMetricsOptions *LeaseMetricsOptions `json:"lease_metrics_options,omitempty" cass-config:"dse:*:lease_metrics_options;recurse"`

	// Configures the schedulers in charge of querying for expired records, removing expired
	// records, and the execution of the checks.
	// +optional
	SolrSchedulerOptions *SolrSchedulerOptions `json:"ttl_index_rebuild_options,omitempty" cass-config:"dse:*:ttl_index_rebuild_options;recurse"`

	// Available options for CQL Solr queries.
	// +optional
	SolrCqlQueryOptions *SolrCqlQueryOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Available options for Solr indexes and indexing operations.
	// +optional
	SolrIndexingOptions *SolrIndexingOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Fault tolerance options for internode communication between DSE Search nodes.
	// +optional
	SolrShardTransportOptions *SolrShardTransportOptions `json:"shard_transport_options,omitempty" cass-config:"dse:*:shard_transport_options;recurse"`

	// Configures the collection of performance metrics on transactional nodes.
	// +optional
	PerformanceServiceOptions *PerformanceServiceOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Configures the behavior of the DSE Analytics nodes.
	AnalyticsOptions *AnalyticsOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Configures the AlwaysOn SQL server on analytics nodes.
	// +optional
	AlwaysOnSqlOptions *AlwaysOnSqlOptions `json:"alwayson_sql_options,omitempty" cass-config:"dse:*:alwayson_sql_options;recurse"`

	// Configures DSEFS (DSE Fielsystem).
	// +optional
	DsefsOptions *DsefsOptions `json:"dsefs_options,omitempty" cass-config:"dse:*:dsefs_options;recurse"`

	// Options for DSE Metrics Collector (Insights).
	// +optional
	InsightsOptions *InsightsOptions `json:"insights_options,omitempty" cass-config:"dse:*:insights_options;recurse"`

	// Configures database activity logging.
	// +optional
	AuditLoggingOptions *AuditLoggingOptions `json:"audit_logging_options,omitempty" cass-config:"dse:*:audit_logging_options;recurse"`

	// Configures the smart movement of data across different types of storage media so that data is
	// matched to the most suitable drive type, according to the required performance and cost
	// characteristics. Each key must be the name of a disk configuration strategy.
	// +optional
	TieredStorageOptions map[string]TieredStorageOptions `json:"tiered_storage_options,omitempty" cass-config:"dse:*:tiered_storage_options;recurse"`

	// Configure DSE Advanced Replication.
	// +optional
	AdvancedReplicationOptions *AdvancedReplicationOptions `json:"advanced_replication_options,omitempty" cass-config:"dse:*:advanced_replication_options;recurse"`

	// Configures the internal messaging service used by several components of DataStax Enterprise.
	// All internode messaging requests use this service.
	// +optional
	InternodeMessagingOptions *InternodeMessagingOptions `json:"internode_messaging_options,omitempty" cass-config:"dse:*:internode_messaging_options;recurse"`

	// System-level configuration options for DSE Graph.
	// +optional
	GraphOptions *GraphOptions `json:"graph,omitempty" cass-config:"dse:*:graph;recurse"`

	// Unique generated ID of the physical server in DSE Multi-Instance /etc/dse-nodeId/dse.yaml
	// files. You can change server_id when the MAC address is not unique, such as a virtualized
	// server where the host’s physical MAC is cloned.
	// Default: the media access control address (MAC address) of the physical server
	// +optional
	// FIXME not implemented yet in config-builder
	ServerId *string `json:"server_id,omitempty" cass-config:"dse:*:server_id"`

	// FIXME not present in the official documentation
	// FIXME present in config-builder, apparently related to backup_service, but does not seem to be recognized by DSE
	// +optional
	// IndexOptions *IndexOptions `json:"index_options,omitempty" cass-config:"dse:*:index_options;recurse"`
}

type AuthenticationOptions struct {

	// Enables user authentication.
	// Default: true.
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The first scheme to validate a user against when the driver does not request a specific
	// scheme. Valid values are:
	// - internal: Plain text authentication using the internal password authentication.
	// - ldap: Plain text authentication using pass-through LDAP authentication.
	// - kerberos: GSSAPI authentication using the Kerberos authenticator.
	// Default: internal.
	// +kubebuilder:validation:Enum=internal;kerberos;ldap
	// +optional
	DefaultScheme *string `json:"default_scheme,omitempty" cass-config:"dse:*:default_scheme"`

	// List of schemes that are checked if validation against the first scheme fails and no scheme
	// was specified by the driver.
	// +kubebuilder:validation:Enum=kerberos;ldap
	// +optional
	OtherSchemes []string `json:"other_schemes,omitempty" cass-config:"dse:*:other_schemes"`

	// Determines if roles need to have permission granted to them to use specific authentication
	// schemes. These permissions can be granted only when the DseAuthorizer is used.
	// - true: Use multiple schemes for authentication. To be assigned, every role requires
	//   permissions to a scheme.
	// - false: Do not use multiple schemes for authentication. Prevents unintentional role
	//   assignment that might occur if user or group names overlap in the authentication
	//   service.
	// Default: false.
	// +optional
	SchemePermissions *bool `json:"scheme_permissions,omitempty" cass-config:"dse:*:scheme_permissions"`

	// Controls whether DIGEST-MD5 authentication is allowed with Kerberos. Kerberos uses
	// DIGEST-MD5 to pass credentials between nodes and jobs. The DIGEST-MD5 mechanism is not
	// associated directly with an authentication scheme.
	// - true: Allow DIGEST-MD5 authentication with Kerberos. In analytics clusters, set to true to
	//   use Hadoop internode authentication with Hadoop and Spark jobs.
	// - false: Do not allow DIGEST-MD5 authentication with Kerberos.
	// Default: true.
	// +optional
	AllowDigestWithKerberos *bool `json:"allow_digest_with_kerberos,omitempty" cass-config:"dse:*:allow_digest_with_kerberos"`

	// Controls how the DseAuthenticator responds to plain text authentication requests over
	// unencrypted client connections.
	// - block: Block the request with an authentication error.
	// - warn: Log a warning but allow the request.
	// - allow: Allow the request without any warning.
	// Default: warn.
	// +kubebuilder:validation:Enum=block;warn;allow
	// +optional
	PlainTextWithoutSsl *string `json:"plain_text_without_ssl,omitempty" cass-config:"dse:*:plain_text_without_ssl"`

	// Sets transitional mode for temporary use during authentication setup in an established
	// environment. Transitional mode allows access to the database using the anonymous role, which
	// has all permissions except AUTHORIZE.
	// - disabled: Disable transitional mode. All connections must provide valid credentials and map
	//   to a login-enabled role.
	// - permissive: Only superusers are authenticated and logged in. All other authentication
	//   attempts are logged in as the anonymous user.
	// - normal: Allow all connections that provide credentials. Maps all authenticated users to
	//   their role, and maps all other connections to anonymous.
	// - strict: Allow only authenticated connections that map to a login-enabled role OR connections
	//   that provide a blank username and password as anonymous.
	// Default: disabled.
	// +kubebuilder:validation:Enum=disabled;permissive;normal;strict
	// +optional
	TransitionalMode *string `json:"transitional_mode,omitempty" cass-config:"dse:*:transitional_mode"`
}

type RoleManagementOptions struct {

	// Manages granting and revoking of roles.
	// - internal: Manage granting and revoking of roles internally using the GRANT ROLE and REVOKE
	//   ROLE CQL statements. See Managing database access. Internal role management allows
	//   nesting roles for permission management.
	// - ldap: Manage granting and revoking of roles using an external LDAP server configured using
	//   the ldap_options. To configure an LDAP scheme, complete the steps in Defining an LDAP
	//   scheme. Nesting roles for permission management is disabled.
	// Default: internal.
	// +kubebuilder:validation:Enum=internal;ldap
	// +optional
	Mode *string `json:"mode,omitempty" cass-config:"dse:*:mode"`

	// Controls the granting and revoking of roles based on the users' authentication method.
	// When this setting is used, it overrides the value from the mode setting.
	// Introduced in DSE 6.8.9.
	// +optional
	ModeByAuthentication *ModeByAuthentication `json:"mode_by_authentication,omitempty" cass-config:"dse:>=6.8.9:mode_by_authentication"`

	// Set to true to enable logging of DSE role creation and modification events in the
	// dse_security.role_stats system table. All nodes must have the stats option enabled, and must
	// be restarted for the functionality to take effect.
	// Introduced in DSE 6.8.1.
	// Default: false.
	// +optional
	Stats *bool `json:"stats,omitempty" cass-config:"dse:>=6.8.1:stats"`
}

type ModeByAuthentication struct {

	// Mode for internal authentication scheme. Allowed values are 'internal' or 'ldap'.
	// Introduced in DSE 6.8.9.
	// +kubebuilder:validation:Enum=internal;ldap
	// +optional
	Internal *string `json:"internal,omitempty" cass-config:"dse:>=6.8.9:internal"`

	// Mode for ldap authentication scheme. Allowed values are 'internal' or 'ldap'.
	// Introduced in DSE 6.8.9.
	// +kubebuilder:validation:Enum=internal;ldap
	// +optional
	Ldap *string `json:"ldap,omitempty" cass-config:"dse:>=6.8.9:ldap"`

	// Mode for kerberos authentication scheme.  Allowed values are 'internal' or 'ldap'.
	// Introduced in DSE 6.8.9.
	// +kubebuilder:validation:Enum=internal;ldap
	// +optional
	Kerberos *string `json:"kerberos,omitempty" cass-config:"dse:>=6.8.9:kerberos"`
}

type AuthorizationOptions struct {

	// Enables the DSE Authorizer for role-based access control (RBAC).
	// - true: Enable the DSE Authorizer for RBAC.
	// - false: Do not use the DSE Authorizer.
	// Default: false.
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// Allows the DSE Authorizer to operate in a temporary mode during authorization setup in a
	// cluster.
	// - disabled: Transitional mode is disabled.
	// - normal: Permissions can be passed to resources, but are not enforced.
	// - strict: Permissions can be passed to resources, and are enforced on authenticated users.
	//   Permissions are not enforced against anonymous users.
	// Default: disabled.
	// +kubebuilder:validation:Enum=disabled;normal;strict
	// +optional
	TransitionalMode *string `json:"transitional_mode,omitempty" cass-config:"dse:*:transitional_mode"`

	// Enables row-level access control (RLAC) permissions. Use the same setting on all nodes. See
	// Setting up Row Level Access Control (RLAC).
	// - true: Use row-level security.
	// - false: Do not use row-level security.
	// Default: false.
	// +optional
	AllowRowLevelSecurity *bool `json:"allow_row_level_security,omitempty" cass-config:"dse:*:allow_row_level_security"`
}

type KerberosOptions struct {

	// The filepath of dse.keytab.
	// Tarball Default: resources/dse/conf/dse.keytab
	// Default value: /etc/dse/conf/dse.keytab
	// TODO mountable directory
	// +optional
	Keytab *string `json:"keytab,omitempty" cass-config:"dse:*:keytab"`

	// The service_principal that the DataStax Enterprise process runs under must use the form
	// dse_user/_HOST@REALM, where:
	// - dse_user is the username of the user that starts the DataStax Enterprise process.
	// - _HOST is converted to a reverse DNS lookup of the broadcast address.
	// - REALM is the name of your Kerberos realm. In the Kerberos principal, REALM must be
	//   uppercase.
	// Default: dse/_HOST@REALM
	// +optional
	ServicePrincipal *string `json:"service_principal,omitempty" cass-config:"dse:*:service_principal"`

	// Used by the Tomcat application container to run DSE Search. The Tomcat web server uses the
	// GSSAPI mechanism (SPNEGO) to negotiate the GSSAPI security mechanism (Kerberos). REALM is the
	// name of your Kerberos realm. In the Kerberos principal, REALM must be uppercase.
	// Default: HTTP/_HOST@REALM
	// +optional
	HTTPPrincipal *string `json:"http_principal,omitempty" cass-config:"dse:*:http_principal"`

	// A comma-delimited list of Quality of Protection (QOP) values that clients and servers can use
	// for each connection. The client can have multiple QOP values, while the server can have only
	// a single QOP value.
	// - auth: Authentication only.
	// - auth-int: Authentication plus integrity protection for all transmitted data.
	// - auth-conf: Authentication plus integrity protection and encryption of all transmitted
	//   data.
	// Encryption using auth-conf is separate and independent of whether encryption is done using
	// SSL. If both auth-conf and SSL are enabled, the transmitted data is encrypted twice. DataStax
	// recommends choosing only one method and using it for encryption and authentication.
	// Default: auth
	// +kubebuilder:validation:Enum=auth;auth-int;auth-conf
	// +optional
	QOP *string `json:"qop,omitempty" cass-config:"dse:*:qop"`
}

type LdapOptions struct {

	// A comma separated list of LDAP server hosts. Important: Do not use LDAP on the same host
	// (localhost) in production environments. Using LDAP on the same host (localhost) is
	// appropriate only in single node test or development environments.
	// Default: none
	// +optional
	ServerHost *string `json:"server_host,omitempty" cass-config:"dse:*:server_host"`

	// The port on which the LDAP server listens.
	// - 389: The default port for unencrypted connections.
	// - 636: Used for encrypted connections. Default SSL or TLS port for LDAP.
	// Default: 389
	// +optional
	ServerPort *int `json:"server_port,omitempty" cass-config:"dse:*:server_port"`

	// Enable hostname verification. The following conditions must be met:
	// - Either use_ssl or use_tls must be set to true.
	// - A valid truststore with the correct path specified in truststore_path must exist.
	//   The truststore must have a certificate entry, trustedCertEntry, including a SAN DNSName
	//   entry that matches the hostname of the LDAP server.
	// Default: false
	// +optional
	HostnameVerification *bool `json:"hostname_verification,omitempty" cass-config:"dse:*:hostname_verification"`

	// Distinguished name (DN) of an account with read access to the user_search_base and
	// group_search_base. For example:
	// - OpenLDAP: uid=lookup,ou=users,dc=springsource,dc=com
	// - Microsoft Active Directory (AD): cn=lookup, cn=users, dc=springsource, dc=com
	// WARNING: Do not create/use an LDAP account or group called cassandra. The DSE database comes
	// with a default cassandra login role that has access to all database objects and uses the
	// consistency level QUOROM.
	// When not set, the LDAP server uses an anonymous bind for search.
	// Default: none
	// +optional
	SearchDN *string `json:"search_dn,omitempty" cass-config:"dse:*:search_dn"`

	// The password of the search_dn account.
	// Default: none
	// +optional
	SearchPassword *string `json:"search_password,omitempty" cass-config:"dse:*:search_password"`

	// Enables an SSL-encrypted connection to the LDAP server.
	// - true: Use an SSL-encrypted connection.
	// - false: Do not enable SSL connections to the LDAP server.
	// Default: false
	// +optional
	UseSSL *bool `json:"use_ssl,omitempty" cass-config:"dse:*:use_ssl"`

	// Enables TLS connections to the LDAP server.
	// - true: Enable TLS connections to the LDAP server.
	// - false: Do not enable TLS connections to the LDAP server
	// Default: false
	// +optional
	UseTLS *bool `json:"use_tls,omitempty" cass-config:"dse:*:use_tls"`

	// The filepath to the SSL certificates truststore.
	// Default: none
	// +optional
	// TODO mountable directory
	TruststorePath *string `json:"truststore_path,omitempty" cass-config:"dse:*:truststore_path"`

	// The password to access the truststore.
	// Default: none
	// +optional
	TruststorePassword *string `json:"truststore_password,omitempty" cass-config:"dse:*:truststore_password"`

	// Valid types are JKS, JCEKS, or PKCS12.
	// Default: JKS
	// +optional
	TruststoreType *string `json:"truststore_type,omitempty" cass-config:"dse:*:truststore_type"`

	// Distinguished name (DN) of the object to start the recursive search for user entries for
	// authentication and role management memberof searches.
	// For your LDAP domain, set the ou and dc elements. Typically set to
	// ou=users,dc=domain,dc=top_level_domain. For example, ou=users,dc=example,dc=com.
	// For your Active Directory, set the dc element for a different search base. Typically set to
	// CN=search,CN=Users,DC=ActDir_domname,DC=internal. For example,
	// CN=search,CN=Users,DC=example-sales,DC=internal.
	// Default: none
	// +optional
	UserSearchBase *string `json:"user_search_base,omitempty" cass-config:"dse:*:user_search_base"`

	// Identifies the user that the search filter uses for looking up usernames.
	// - (uid={0}): When using LDAP.
	// - (samAccountName={0}): When using AD (Microsoft Active Directory).
	//   For example, (sAMAccountName={0}).
	// Default: (uid={0})
	// +optional
	UserSearchFilter *string `json:"user_search_filter,omitempty" cass-config:"dse:*:user_search_filter"`

	// Contains a list of group names. Role manager assigns DSE roles that exactly match any group
	// name in the list. Required when managing roles using group_search_type: memberof_search with
	// LDAP (role_manager.mode:ldap). The directory server must have memberof support, which is a
	// default user attribute in Microsoft Active Directory (AD).
	// Default: memberof
	// +optional
	UserMemberOfAttribute *string `json:"user_memberof_attribute,omitempty" cass-config:"dse:*:user_memberof_attribute"`

	// Option to define additional search bases for users. If the user is not found in one search
	// base, DSE attempts to find the user in another search base, until all search bases have been
	// tried.
	// Default: [] (empty list)
	// Introduced in DSE 6.8.2.
	// +optional
	ExtraUserSearchBases []string `json:"extra_user_search_bases,omitempty" cass-config:"dse:>=6.8.2:extra_user_search_bases"`

	// Defines how group membership is determined for a user. Required when managing roles with
	// LDAP (role_manager.mode: ldap).
	// - directory_search: Filters the results with a subtree search of group_search_base to find
	//   groups that contain the username in the attribute defined in the group_search_filter.
	// - memberof_search: Recursively searches for user entries using the user_search_base and
	//   user_search_filter. Gets groups from the user attribute defined in user_memberof_attribute.
	//   The directory server must have memberof support.
	// Default: directory_search
	// +kubebuilder:validation:Enum=directory_search;memberof_search
	// +optional
	GroupSearchType *string `json:"group_search_type,omitempty" cass-config:"dse:*:group_search_type"`

	// The unique distinguished name (DN) of the group record from which to start the group
	// membership search.
	// Default: none
	// +optional
	GroupSearchBase *string `json:"group_search_base,omitempty" cass-config:"dse:*:group_search_base"`

	// Set to any valid LDAP filter.
	// Default: uniquemember={0}
	// +optional
	GroupSearchFilter *string `json:"group_search_filter,omitempty" cass-config:"dse:*:group_search_filter"`

	// The attribute in the group record that contains the LDAP group name. Role names are
	// case-sensitive and must match exactly on DSE for assignment. Unmatched groups are ignored.
	// Default: cn
	// +optional
	GroupNameAttribute *string `json:"group_name_attribute,omitempty" cass-config:"dse:*:group_name_attribute"`

	// Option to define additional search bases for groups. DSE merges all groups found in all the
	// defined search bases.
	// Default: [] (empty list)
	// Introduced in DSE 6.8.2.
	// +optional
	ExtraGroupSearchBases []string `json:"extra_group_search_bases,omitempty" cass-config:"dse:>=6.8.2:extra_group_search_bases"`

	// A credentials cache improves performance by reducing the number of requests that are sent to
	// the internal or LDAP server. See Defining an LDAP scheme.
	// - 0: Disable credentials cache.
	// - duration period: The duration period in milliseconds of the credentials cache.
	// Note: Starting in DSE 6.8.2, the upper limit for ldap_options.credentials_validity_in_ms
	// increased to 864,000,000 ms, which is 10 days.
	// Default: 0
	// +optional
	CredentialsValidityInMs *int `json:"credentials_validity_in_ms,omitempty" cass-config:"dse:*:credentials_validity_in_ms"`

	// Configures a search cache to improve performance by reducing the number of requests that are
	// sent to the internal or LDAP server.
	// - 0: Disables search credentials cache.
	// - positive number: The duration period in seconds for the search cache.
	// Note: Starting in DSE 6.8.2, the upper limit for ldap_options.search_validity_in_seconds
	// increased to 864,000 seconds, which is 10 days.
	// Default: 0
	// +optional
	SearchValidityInSeconds *int `json:"search_validity_in_seconds,omitempty" cass-config:"dse:*:search_validity_in_seconds"`

	// Configures the connection pool for making LDAP requests.
	// +optional
	ConnectionPool *LdapConnectionPoolOptions `json:"connection_pool,omitempty" cass-config:"dse:*:connection_pool;recurse"`

	// Introduced in DSE 6.8.1; not present in the official docs.
	// Default: TLS
	// +optional
	SslProtocol *string `json:"ssl_protocol,omitempty" cass-config:"dse:>=6.8.1:ssl_protocol"`

	// Introduced in DSE 6.8.2; not present in the official docs.
	// +optional
	// +kubebuilder:validation:Enum=default;directory_search;memberof_search
	AllParentGroupsSearchType *string `json:"all_parent_groups_search_type,omitempty" cass-config:"dse:>=6.8.2:all_parent_groups_search_type"`

	// Introduced in DSE 6.8.2; not present in the official docs.
	// +optional
	AllParentGroupsMemberOfAttribute *string `json:"all_parent_groups_memberof_attribute,omitempty" cass-config:"dse:>=6.8.2:all_parent_groups_memberof_attribute"`

	// Introduced in DSE 6.8.2; not present in the official docs.
	// +optional
	AllParentGroupsSearchFilter *string `json:"all_parent_groups_search_filter,omitempty" cass-config:"dse:>=6.8.2:all_parent_groups_search_filter"`

	// Introduced in DSE 6.8.4; not present in the official docs.
	// +optional
	DnsServiceDiscovery *DnsServiceDiscovery `json:"dns_service_discovery,omitempty" cass-config:"dse:>=6.8.4:dns_service_discovery;recurse"`

	// Validity period for the groups cache in milliseconds.
	// Introduced in DSE 6.8.5; not present in the official docs.
	// Default: 0
	// +optional
	GroupsValidityInMs *int `json:"groups_validity_in_ms,omitempty" cass-config:"dse:>=6.8.5:groups_validity_in_ms"`

	// Refresh interval for groups cache (if enabled). After this interval, cache entries become
	// eligible for refresh.
	// Introduced in DSE 6.8.5; not present in the official docs.
	// Default: 0
	// +optional
	GroupsUpdateIntervalInMs *int `json:"groups_update_interval_in_ms,omitempty" cass-config:"dse:>=6.8.5:groups_update_interval_in_ms"`
}

type LdapConnectionPoolOptions struct {

	// The maximum number of active connections to the LDAP server.
	// Default: 8
	// +optional
	MaxActive *int `json:"max_active,omitempty" cass-config:"dse:*:max_active"`

	// The maximum number of idle connections in the pool awaiting requests.
	// Default: 8
	// +optional
	MaxIdle *int `json:"max_idle,omitempty" cass-config:"dse:*:max_idle"`
}

type DnsServiceDiscovery struct {

	// Fully qualified domain name to get the SRV records from.
	// Introduced in DSE 6.8.4; not present in the official docs.
	// +optional
	Fqdn *string `json:"fqdn,omitempty" cass-config:"dse:>=6.8.4:fqdn"`

	// Timeout in ms for querying DNS; it has to be between 0 and 1 hour equivalent.
	// Introduced in DSE 6.8.4; not present in the official docs.
	// +optional
	// Default: 5000
	LookupTimeoutMs *int `json:"lookup_timeout_ms,omitempty" cass-config:"dse:>=6.8.4:lookup_timeout_ms"`

	// For how long the old results should be retained/cached (value between 0 and 10 days
	// equivalent).
	// Introduced in DSE 6.8.4; not present in the official docs.
	// Default: 600000
	// +optional
	RetentionDurationMs *int `json:"retention_duration_ms,omitempty" cass-config:"dse:>=6.8.4:retention_duration_ms"`

	// How often DSE should try to refresh the list of obtained servers.
	// Introduced in DSE 6.8.4; not present in the official docs.
	// Default: 0
	// +optional
	PollingIntervalMs *int `json:"polling_interval_ms,omitempty" cass-config:"dse:>=6.8.4:polling_interval_ms"`
}

type SystemInfoEncryptionOptions struct {

	// Enables encryption of system resources. Note: The system_trace keyspace is not encrypted by
	// enabling the system_information_encryption section. In environments that also have tracing
	// enabled, manually configure encryption with compression on the system_trace keyspace.
	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The name of the JCE cipher algorithm used to encrypt system resources.
	// Supported cipher algorithms names and strengths are:
	// - AES: 128, 192, or 256
	// - DES: 56
	// - DESede: 112 or 168
	// - Blowfish: 32-448
	// - RC2: 40-128
	// Default: AES
	// +optional
	// +kubebuilder:validation:Enum=AES;DES;DESede;Blowfish;RC2
	CipherAlgorithm *string `json:"cipher_algorithm,omitempty" cass-config:"dse:*:cipher_algorithm"`

	// Length of key to use for the system resources.
	// Note: DSE uses a matching local key or requests the key type from the KMIP server. For KMIP,
	// if an existing key does not match, the KMIP server automatically generates a new key.
	// Default: 128
	// +optional
	SecretKeyStrength *int `json:"secret_key_strength,omitempty" cass-config:"dse:*:secret_key_strength"`

	// Optional. Size of SSTable chunks when data from the system.batchlog or system.paxos are
	// written to disk.
	// Note: To encrypt existing data, run nodetool upgradesstables -a system batchlog paxos on all
	// nodes in the cluster.
	// Default: 64
	// +optional
	ChunkLengthKb *int `json:"chunk_length_kb,omitempty" cass-config:"dse:*:chunk_length_kb"`

	// KMIP key provider to enable encrypting sensitive system data with a KMIP key. Comment out if
	// using a local encryption key.
	// Default: KmipKeyProviderFactory
	// +optional
	KeyProvider *string `json:"key_provider,omitempty" cass-config:"dse:*:key_provider"`

	// The KMIP key server host. Set to the kmip_group_name that defines the KMIP host in kmip_hosts
	// section. DSE requests a key from the KMIP host and uses the key generated by the KMIP
	// provider.
	// Default: kmip_host_name
	// +optional
	KmipHost *string `json:"kmip_host,omitempty" cass-config:"dse:*:kmip_host"`
}

type ConfigurationEncryptionOptions struct {

	// Path to the directory where local encryption key files are stored, also called system keys.
	// Distributes the system keys to all nodes in the cluster. Ensure the DSE account is the folder
	// owner and has read/write/execute (700) permissions. Note: This directory is not used for KMIP
	// keys.
	// Default: /etc/dse/conf
	// +optional
	// TODO mountable directory
	SystemKeyDirectory *string `json:"system_key_directory,omitempty" cass-config:"dse:*:system_key_directory"`

	// Enables encryption on sensitive data stored in tables and in configuration files.
	// - true: Enable encryption of configuration property values using the specified
	//   config_encryption_key_name. When set to true, the configuration values must be encrypted or
	//   commented out. Restriction: Lifecycle Manager (LCM) is not compatible when
	//   config_encryption_active is true in DSE and OpsCenter.
	// - false: Do not enable encryption of configuration property values.
	// Default: false
	// +optional
	ConfigEncryptionActive *bool `json:"config_encryption_active,omitempty" cass-config:"dse:*:config_encryption_active"`

	// The local encryption key filename or KMIP key URL to use for configuration file property
	// value decryption. Note: Use dsetool encryptconfigvalue to generate encrypted values for the
	// configuration file properties.
	// Default: system_key
	// Note: The default name is not configurable.
	// +optional
	ConfigEncryptionKeyName *string `json:"config_encryption_key_name,omitempty" cass-config:"dse:*:config_encryption_key_name"`
}

type KmipEncryptionOptions struct {

	// Each key in the map is a user-defined name for a group of options to configure a KMIP server
	// or servers, key settings, and certificates. For each KMIP key server or group of KMIP key
	// servers, you must configure options for a kmip_groupname section.
	// +optional
	KmipHosts map[string]KmipHostsGroup `json:"kmip_hosts,omitempty" cass-config:"dse:*:kmip_hosts;recurse"`
}

type KmipHostsGroup struct {

	// A comma-separated list of KMIP hosts (host[:port]) using the FQDN (Fully Qualified Domain
	// Name). Add KMIP hosts in the intended failover sequence because DSE queries the host in the
	// listed order.
	// +optional
	Hosts *string `json:"hosts,omitempty" cass-config:"dse:*:hosts"`

	// The path to a Java keystore created from the KMIP agent PEM files.
	// Default: /etc/dse/conf/KMIP_keystore.jks
	// +optional
	// TODO mountable directory
	KeystorePath *string `json:"keystore_path,omitempty" cass-config:"dse:*:keystore_path"`

	// Valid types are JKS, JCEKS, PKCS11, and PKCS12. For file-based keystores, use PKCS12.
	// Default: JKS
	// +optional
	// +kubebuilder:validation:Enum=JKS;JCEKS;PKCS11;PKCS12
	KeystoreType *string `json:"keystore_type,omitempty" cass-config:"dse:*:keystore_type"`

	// Password used to protect the private key of the key pair.
	// Default: none
	// +optional
	KeystorePassword *string `json:"keystore_password,omitempty" cass-config:"dse:*:keystore_password"`

	// The path to a Java truststore that was created using the KMIP root certificate.
	// Default: /etc/dse/conf/KMIP_truststore.jks
	// +optional
	// TODO mountable directory
	TruststorePath *string `json:"truststore_path,omitempty" cass-config:"dse:*:truststore_path"`

	// Valid types are JKS, JCEKS, PKCS12. For file-based truststores, use PKCS12.
	// Attention: Due to an OpenSSL issue, you cannot use a PKCS12 truststore that was generated via
	// OpenSSL. However, truststores generated via Java's keytool and then converted to PKCS12 work
	// with DSE.
	// Default: JKS
	// +optional
	// +kubebuilder:validation:Enum=JKS;JCEKS;PKCS12
	TruststoreType *string `json:"truststore_type,omitempty" cass-config:"dse:*:truststore_type"`

	// Password required to access the keystore.
	// Default: none
	// +optional
	TruststorePassword *string `json:"truststore_password,omitempty" cass-config:"dse:*:truststore_password"`

	// Milliseconds to locally cache the encryption keys that are read from the KMIP hosts. The
	// longer the encryption keys are cached, the fewer requests to the KMIP key server are made
	// and the longer it takes for changes, like revocation, to propagate to the DSE node. DataStax
	// Enterprise uses concurrent encryption, so multiple threads fetch the secret key from the KMIP
	// key server at the same time. DataStax recommends using the default value.
	// Default: 300000
	// +optional
	KeyCacheMillis *int `json:"key_cache_millis,omitempty" cass-config:"dse:*:key_cache_millis"`

	// Socket timeout in milliseconds.
	// Default: 1000
	// +optional
	TimeoutMillis *int `json:"timeout,omitempty" cass-config:"dse:*:timeout"`

	// Refresh interval for the KMIP host key cache. After this interval, cache entries become
	// eligible for refresh. Upon next access, an async reload is scheduled and the old value
	// returned until it completes. If key_cache_millis is non-zero, then this must be also.
	// Defaults to the same value as key_cache_millis.
	// Introduced in DSE 6.8.11; not present in the official docs.
	// +optional
	KeyCacheUpdateMillis *int `json:"key_cache_update_millis,omitempty" cass-config:"dse:>=6.8.11:key_cache_update_millis"`
}

type SolrEncryptionOptions struct {

	// Allocates shared DSE Search decryption cache off JVM heap.
	// - true: Allocate shared DSE Search decryption cache off JVM heap.
	// - false: Do not allocate shared DSE Search decryption cache off JVM heap.
	// Default: true
	// +optional
	DecryptionCacheOffheapAllocation *bool `json:"decryption_cache_offheap_allocation,omitempty" cass-config:"dse:*:decryption_cache_offheap_allocation"`

	// The maximum size of the shared DSE Search decryption cache in megabytes (MB).
	// Default: 256
	// +optional
	DecryptionCacheSizeInMb *int `json:"decryption_cache_size_in_mb,omitempty" cass-config:"dse:*:decryption_cache_size_in_mb"`
}

type DseInMemoryOptions struct {

	// A fraction of the system memory. For example, 0.20 allows use up to 20% of system memory.
	// This setting is ignored if max_memory_to_lock_mb is set to a non-zero value.
	// Default: 0.20
	// +optional
	MaxMemoryToLockFraction *resource.Quantity `json:"max_memory_to_lock_fraction,omitempty" cass-config:"dse:*:max_memory_to_lock_fraction"`

	// Maximum amount of memory in megabytes (MB) for DSE In-Memory tables.
	// - not set: Use the fraction specified with max_memory_to_lock_fraction.
	// - number greater than 0: Maximum amount of memory in megabytes (MB).
	// Default: 10240
	// +optional
	MaxMemoryToLockMb *int `json:"max_memory_to_lock_mb,omitempty" cass-config:"dse:*:max_memory_to_lock_mb"`
}

type NodeHealthOptions struct {

	// How frequently statistics update.
	// Default: 60000
	// +optional
	RefreshRateMs *int `json:"refresh_rate_ms,omitempty" cass-config:"dse:*:refresh_rate_ms"`

	// The amount of continuous uptime required for the node's uptime score to advance the node
	// health score from 0 to 1 (full health), assuming there are no recent dropped mutations. The
	// health score is a composite score based on dropped mutations and uptime.
	// Tip: If a node is repairing after a period of downtime, increase the uptime period to the
	// expected repair time.
	// Default: 10800 (3 hours)
	// +optional
	UptimeRampUpPeriodSeconds *int `json:"uptime_ramp_up_period_seconds,omitempty" cass-config:"dse:*:uptime_ramp_up_period_seconds"`

	// The historic time window over which the rate of dropped mutations affects the node health
	// score.
	// Default: 30
	// +optional
	DroppedMutationWindowMinutes *int `json:"dropped_mutation_window_minutes,omitempty" cass-config:"dse:*:dropped_mutation_window_minutes"`
}

type LeaseMetricsOptions struct {

	// Enables log entries related to lease holders.
	// - true: Enable log entries related to lease holders to help monitor performance of the lease
	//   subsystem.
	// - false: No not enable log entries.
	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// ttl_seconds
	// Time interval in milliseconds to persist the log of lease holder changes.
	// Default: 604800
	// +optional
	TtlSeconds *int `json:"ttl_seconds,omitempty" cass-config:"dse:*:ttl_seconds"`
}

type SolrIndexingOptions struct {

	// The directory to store index data. See Managing the location of DSE Search data. By default,
	// each DSE Search index is saved in solr_data_dir/keyspace_name.table_name or as specified by
	// the dse.solr.data.dir system property.
	// Default: A solr.data directory in the cassandra data directory, like /var/lib/cassandra/solr.data.
	// TODO mountable directory
	DataDir *string `json:"solr_data_dir,omitempty" cass-config:"dse:*:solr_data_dir"`

	// The Apache Lucene® field cache is deprecated. Instead, for fields that are sorted, faceted,
	// or grouped by, set docValues="true" on the field in the search index schema. Then reload the
	// search index and reindex.
	// Default: false
	FieldCacheEnabled *bool `json:"solr_field_cache_enabled,omitempty" cass-config:"dse:*:solr_field_cache_enabled"`

	// The maximum number of queued partitions during search index rebuilding and reindexing. This
	// maximum number safeguards against excessive heap use by the indexing queue. If set lower than
	// the number of threads per core (TPC), not all TPC threads can be actively indexing.
	// Default: 1024
	// +optional
	BackPressureThresholdPerCore *int `json:"back_pressure_threshold_per_core,omitempty" cass-config:"dse:*:back_pressure_threshold_per_core"`

	// The maximum time, in minutes, to wait for the flushing of asynchronous index updates that
	// occurs at DSE Search commit time or at flush time.
	// CAUTION: Expert knowledge is required to change this value.
	// Always set the wait time high enough to ensure flushing completes successfully to fully sync
	// DSE Search indexes with the database data. If the wait time is exceeded, index updates are
	// only partially committed and the commit log is not truncated which can undermine data
	// durability.
	// Note: When a timeout occurs, this node is typically overloaded and cannot flush in a timely
	// manner. Live indexing increases the time to flush asynchronous index updates.
	// Default: 5
	// +optional
	FlushMaxTimePerCoreMinutes *int `json:"flush_max_time_per_core,omitempty" cass-config:"dse:*:flush_max_time_per_core"`

	// The maximum time, in minutes, to wait for each DSE Search index to load on startup or
	// create/reload operations. This advanced option should be changed only if exceptions happen
	// during search index loading.
	// Default: 5
	// +optional
	LoadMaxTimePerCoreMinutes *int `json:"load_max_time_per_core,omitempty" cass-config:"dse:*:load_max_time_per_core"`

	// Whether to apply the configured disk failure policy if IOExceptions occur during index update
	// operations.
	// - true: Apply the configured Cassandra disk failure policy to index write failures.
	// - false: Do not apply the disk failure policy.
	// Default: false
	// +optional
	EnableIndexDiskFailurePolicy *bool `json:"enable_index_disk_failure_policy,omitempty" cass-config:"dse:*:enable_index_disk_failure_policy"`

	// Global Lucene RAM buffer usage threshold for heap to force segment flush. Setting too low can
	// cause a state of constant flushing during periods of ongoing write activity. For
	// near-real-time (NRT) indexing, forced segment flushes also de-schedule pending auto-soft
	// commits to avoid potentially flushing too many small segments.
	// Default: 1024
	// +optional
	RamBufferHeapSpaceInMb *int `json:"ram_buffer_heap_space_in_mb,omitempty" cass-config:"dse:*:ram_buffer_heap_space_in_mb"`

	// Global Lucene RAM buffer usage threshold for offheap to force segment flush. Setting too low
	// can cause a state of constant flushing during periods of ongoing write activity. For NRT,
	// forced segment flushes also de-schedule pending auto-soft commits to avoid potentially
	// flushing too many small segments. When not set, the default is 1024.
	// Default: 1024
	// +optional
	RamBufferOffheapSpaceInMb *int `json:"ram_buffer_offheap_space_in_mb,omitempty" cass-config:"dse:*:ram_buffer_offheap_space_in_mb"`

	// For DSE Search, configure whether to asynchronously reindex bootstrapped data.
	// - true: The node joins the ring immediately after bootstrap and reindexing occurs
	//   asynchronously. Do not wait for post-bootstrap reindexing so that the node is not marked
	//   down. The dsetool ring command can be used to check the status of the reindexing.
	// - false: The node joins the ring after reindexing the bootstrapped data.
	// Default: false
	// +optional
	AsyncBootstrapReindex *bool `json:"async_bootstrap_reindex,omitempty" cass-config:"dse:*:async_bootstrap_reindex"`

	// Configures the maximum file size of the search index config or schema. Resource files can be
	// uploaded, but the search index config and schema are stored internally in the database after
	// upload.
	// - 0: Disable resource uploading.
	// - upload size: The maximum upload size limit in megabytes (MB) for a DSE Search resource
	//   file (search index config or schema).
	// Default: 10
	// +optional
	ResourceUploadLimitMb *int `json:"solr_resource_upload_limit_mb,omitempty" cass-config:"dse:*:solr_resource_upload_limit_mb"`
}

type SolrCqlQueryOptions struct {

	// Possible values:
	// - driver: Respects driver paging settings. Uses Solr pagination (cursors) only when the
	//   driver uses pagination. Enabled automatically for DSE SearchAnalytics workloads.
	// - off: Paging is off. Ignore driver paging settings for CQL queries and use normal Solr
	//   paging unless:
	//   - The current workload is an analytics workload, including SearchAnalytics.
	//     SearchAnalytics nodes always use driver paging settings.
	//   - The cqlsh query parameter paging is set to driver.
	//   Even when cql_solr_query_paging: off, paging is dynamically enabled with the
	//   "paging":"driver" parameter in JSON queries.
	// Default: off
	// +kubebuilder:validation:Enum=driver;off
	// +optional
	Paging *string `json:"cql_solr_query_paging,omitempty" cass-config:"dse:*:cql_solr_query_paging"`

	// The maximum time in milliseconds to wait for all rows to be read from the database during
	// CQL Solr queries.
	// Default: 10000 (10 seconds)
	// +optional
	RowTimeout *int `json:"cql_solr_query_row_timeout,omitempty" cass-config:"dse:*:cql_solr_query_row_timeout"`
}

type SolrSchedulerOptions struct {

	// Time interval in seconds to check for expired data in seconds.
	// Default: 300
	// +optional
	FixedRatePeriodInSeconds *int `json:"fixed_rate_period,omitempty" cass-config:"dse:*:fixed_rate_period"`

	// The number of seconds to delay the first TTL check to speed up start-up time.
	// Default: 20
	// +optional
	InitialDelayInSeconds *int `json:"initial_delay,omitempty" cass-config:"dse:*:initial_delay"`

	// The maximum number of documents to check and delete per batch by the TTL rebuild thread.
	// All expired documents are deleted from the index during each check. To avoid memory pressure,
	// their unique keys are retrieved and then deletes are issued in batches.
	// Default: 4096
	// +optional
	MaxDocsPerBatch *int `json:"max_docs_per_batch,omitempty" cass-config:"dse:*:max_docs_per_batch"`

	// The maximum number of search indexes (cores) that can execute TTL cleanup concurrently.
	// Manages system resource consumption and prevents many search cores from executing
	// simultaneous TTL deletes.
	// Default: 1
	// +optional
	ThreadPoolSize *int `json:"thread_pool_size,omitempty" cass-config:"dse:*:thread_pool_size"`
}

type SolrShardTransportOptions struct {

	// Timeout behavior during distributed queries. The internal timeout for all search queries to
	// prevent long-running queries. The client request timeout is the maximum cumulative time (in
	// milliseconds) that a distributed search request will wait idly for shard responses.
	// Default: 60000 (1 minute)
	NettyClientRequestTimeoutMs *int `json:"netty_client_request_timeout,omitempty" cass-config:"dse:*:netty_client_request_timeout"`
}

type PerformanceServiceOptions struct {

	// Number of background threads used by the performance service under normal conditions.
	// Default: 4
	// +optional
	CoreThreads *int `json:"performance_core_threads,omitempty" cass-config:"dse:*:performance_core_threads"`

	// Maximum number of background threads used by the performance service.
	// Default: 32
	// +optional
	MaxThreads *int `json:"performance_max_threads,omitempty" cass-config:"dse:*:performance_max_threads"`

	// Allowed number of queued tasks in the backlog when the number of performance_max_threads are
	// busy.
	// Default: 32000
	// +optional
	QueueCapacity *int `json:"performance_queue_capacity,omitempty" cass-config:"dse:*:performance_queue_capacity"`

	// Configures collection of information about slow CQL queries.
	// +optional
	CqlSlowLogOptions *CqlSlowLogOptions `json:"cql_slow_log_options,omitempty" cass-config:"dse:*:cql_slow_log_options;recurse"`

	// Configures collection of system-wide performance information about a cluster.
	// +optional
	CqlSystemInfoOptions *MetricsOptions `json:"cql_system_info_options,omitempty" cass-config:"dse:*:cql_system_info_options;recurse"`

	// Configures collection of object I/O performance statistics.
	// +optional
	ResourceLevelLatencyTrackingOptions *MetricsOptions `json:"resource_level_latency_tracking_options,omitempty" cass-config:"dse:*:resource_level_latency_tracking_options;recurse"`

	// Configures collection of summary statistics at the database level.
	// +optional
	DbSummaryStatsOptions *MetricsOptions `json:"db_summary_stats_options,omitempty" cass-config:"dse:*:db_summary_stats_options;recurse"`

	// Configures collection of statistics at a cluster-wide level.
	// +optional
	ClusterSummaryStatsOptions *MetricsOptions `json:"cluster_summary_stats_options,omitempty" cass-config:"dse:*:cluster_summary_stats_options;recurse"`

	// Configures collection of data associated with Spark cluster and Spark applications.
	// +optional
	SparkClusterInfoOptions *MetricsOptions `json:"spark_cluster_info_options,omitempty" cass-config:"dse:*:spark_cluster_info_options;recurse"`

	// Histogram data for the dropped mutation metrics are stored in the dropped_messages table in
	// the dse_perf keyspace.
	// +optional
	HistogramDataOptions *HistogramDataOptions `json:"histogram_data_options,omitempty" cass-config:"dse:*:histogram_data_options;recurse"`

	// User-resource latency tracking settings.
	// +optional
	UserLevelLatencyTrackingOptions *UserLevelLatencyTrackingOptions `json:"user_level_latency_tracking_options,omitempty" cass-config:"dse:*:user_level_latency_tracking_options;recurse"`

	// +optional
	SolrPerformanceOptions *SolrPerformanceOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Collection of Spark application metrics.
	// +optional
	SparkApplicationInfoOptions *SparkApplicationInfoOptions `json:"spark_application_info_options,omitempty" cass-config:"dse:*:spark_application_info_options;recurse"`

	// Graph event information.
	// +optional
	GraphEvents *GraphEvents `json:"graph_events,omitempty" cass-config:"dse:*:graph_events;recurse"`
}

type CqlSlowLogOptions struct {

	// - true: Enables log entries for slow queries.
	// - false: Does not enable log entries.
	// Default: true
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The threshold in milliseconds or as a percentile.
	// A value greater than 1 is expressed in time and will log queries that take longer than the
	// specified number of milliseconds. For example, 200.0 sets the threshold at 0.2 seconds.
	// A value of 0 to 1 is expressed as a percentile and will log queries that exceed this
	// percentile. For example, .95 collects information on 5% of the slowest queries.
	// Default: 200.0
	// +optional
	Threshold *resource.Quantity `json:"threshold,omitempty" cass-config:"dse:*:threshold"`

	// The initial number of queries before activating the percentile filter.
	// Default: 100
	// +optional
	MinimumSamples *int `json:"minimum_samples,omitempty" cass-config:"dse:*:minimum_samples"`

	// Number of seconds a slow log record survives before it is expired.
	// Default: 259200
	// +optional
	TtlSeconds *int `json:"ttl_seconds,omitempty" cass-config:"dse:*:ttl_seconds"`

	// Keeps slow queries only in-memory and does not write data to database.
	// - true: Keep slow queries only in-memory. Skip writing to database.
	// - false: Write slow query information in the node_slow_log table. The threshold must be >=
	// 2000 ms to prevent a high load on the database.
	// Default: true
	// +optional
	SkipWritingToDb *bool `json:"skip_writing_to_db,omitempty" cass-config:"dse:*:skip_writing_to_db"`

	// The number of slow queries to keep in-memory.
	// Default: 5
	// +optional
	NumSlowestQueries *int `json:"num_slowest_queries,omitempty" cass-config:"dse:*:num_slowest_queries"`
}

type SolrPerformanceOptions struct {

	// Configures reporting distributed sub-queries for search (query executions on individual
	// shards) that take longer than a specified period of time.
	// +optional
	SlowSubQueryLogOptions *SolrSlowSubQueryLogOptions `json:"solr_slow_sub_query_log_options,omitempty" cass-config:"dse:*:solr_slow_sub_query_log_options;recurse"`

	// Options to collect search index direct update handler statistics over time.
	// +optional
	UpdateHandlerMetricsOptions *SolrMetricsOptions `json:"solr_update_handler_metrics_options,omitempty" cass-config:"dse:*:solr_update_handler_metrics_options;recurse"`

	// Options to collect search index request handler statistics over time.
	// +optional
	RequestHandlerMetricsOptions *SolrMetricsOptions `json:"solr_request_handler_metrics_options,omitempty" cass-config:"dse:*:solr_request_handler_metrics_options;recurse"`

	// Options to record search index statistics over time.
	// +optional
	IndexStatsOptions *SolrMetricsOptions `json:"solr_index_stats_options,omitempty" cass-config:"dse:*:solr_index_stats_options;recurse"`

	// +optional
	CacheStatsOptions *SolrMetricsOptions `json:"solr_cache_stats_options,omitempty" cass-config:"dse:*:solr_cache_stats_options;recurse"`

	// +optional
	LatencySnapshotOptions *SolrMetricsOptions `json:"solr_latency_snapshot_options,omitempty" cass-config:"dse:*:solr_latency_snapshot_options;recurse"`
}

type SolrSlowSubQueryLogOptions struct {

	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// Default: 604800
	// +optional
	TtlSeconds *int `json:"ttl_seconds,omitempty" cass-config:"dse:*:ttl_seconds"`

	// The number of server threads dedicated to writing in the log. More than one server thread
	// might degrade performance.
	// Default: 1
	// +optional
	AsyncWriters *int `json:"async_writers,omitempty" cass-config:"dse:*:async_writers"`

	// Default: 3000
	// +optional
	ThresholdMs *int `json:"threshold_ms,omitempty" cass-config:"dse:*:threshold_ms"`
}

type MetricsOptions struct {

	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// +optional
	// Default: 60000
	RefreshRateMs *int `json:"refresh_rate_ms,omitempty" cass-config:"dse:*:refresh_rate_ms"`
}

type HistogramDataOptions struct {
	MetricsOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Default: 3
	// +optional
	RetentionCount *int `json:"retention_count,omitempty" cass-config:"dse:*:retention_count"`
}

type UserLevelLatencyTrackingOptions struct {
	MetricsOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// The maximum number of individual metrics.
	// Default: 100
	// +optional
	TopStatsLimit *int `json:"top_stats_limit,omitempty" cass-config:"dse:*:top_stats_limit"`

	// Default: false
	// +optional
	Quantiles *bool `json:"quantiles,omitempty" cass-config:"dse:*:quantiles"`
}

type SolrMetricsOptions struct {
	MetricsOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Default: 10000 (604800 for Solr)
	// +optional
	TtlSeconds *int `json:"ttl_seconds,omitempty" cass-config:"dse:*:ttl_seconds"`
}

type SparkApplicationInfoOptions struct {

	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The length of the sampling period in milliseconds; the frequency to update the performance
	// statistics.
	// Default: 10000 (10 seconds)
	// +optional
	RefreshRateMs *int `json:"refresh_rate_ms,omitempty" cass-config:"dse:*:refresh_rate_ms"`

	// Configures collection of metrics at the Spark Driver.
	// +optional
	Driver *SparkApplicationInfoDriverOptions `json:"driver,omitempty" cass-config:"dse:*:driver;recurse"`

	// Configures collection of metrics at Spark executors.
	// +optional
	Executor *SparkApplicationInfoExecutorOptions `json:"executor,omitempty" cass-config:"dse:*:executor;recurse"`
}

type SparkApplicationInfoDriverOptions struct {
	SparkApplicationInfoExecutorOptions `json:",inline" cass-config:"dse:*:;recurse"`

	// Default: false
	// +optional
	StateSource *bool `json:"stateSource,omitempty" cass-config:"dse:*:stateSource"`
}

type SparkApplicationInfoExecutorOptions struct {

	// Default: false
	// +optional
	Sink *bool `json:"sink,omitempty" cass-config:"dse:*:sink"`

	// Default: false
	// +optional
	ConnectorSource *bool `json:"connectorSource,omitempty" cass-config:"dse:*:connectorSource"`

	// jvmSource
	// Default: false
	// +optional
	JvmSource *bool `json:"jvmSource,omitempty" cass-config:"dse:*:jvmSource"`
}

type GraphEvents struct {

	// Number of seconds a record survives before it is expired.
	// Default: 600
	// +optional
	TtlSeconds *int `json:"ttl_seconds,omitempty" cass-config:"dse:*:ttl_seconds"`
}

type AlwaysOnSqlOptions struct {

	// Enables AlwaysOn SQL for this node.
	// - true: Enable AlwaysOn SQL for this node. The node must be an analytics node. Set workpools
	//   in Spark resource_manager_options.
	// - false: Do not enable AlwaysOn SQL for this node.
	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The Thrift port on which AlwaysOn SQL listens.
	// Default: 10000
	// +optional
	ThriftPort *int `json:"thrift_port,omitempty" cass-config:"dse:*:thrift_port"`

	// The port on which the AlwaysOn SQL web UI is available.
	// Default: 9077
	// +optional
	WebUIPort *int `json:"web_ui_port,omitempty" cass-config:"dse:*:web_ui_port"`

	// The wait time in milliseconds to reserve the thrift_port if it is not available.
	// Default: 100
	// +optional
	ReservePortWaitTimeMs *int `json:"reserve_port_wait_time_ms,omitempty" cass-config:"dse:*:reserve_port_wait_time_ms"`

	// The time in milliseconds to wait for a health check status of the AlwaysOn SQL server.
	// Default: 500
	// +optional
	StatusCheckWaitTimeMs *int `json:"alwayson_sql_status_check_wait_time_ms,omitempty" cass-config:"dse:*:alwayson_sql_status_check_wait_time_ms"`

	// The named workpool used by AlwaysOn SQL.
	// Default: alwayson_sql
	// +optional
	Workpool *string `json:"workpool,omitempty" cass-config:"dse:*:workpool"`

	// Location in DSEFS of the AlwaysOn SQL log files.
	// Default: /spark/log/alwayson_sql
	// +optional
	// TODO mountable directory
	LogDsefsDir *string `json:"log_dsefs_dir,omitempty" cass-config:"dse:*:log_dsefs_dir"`

	// The role to use for internal communication by AlwaysOn SQL if authentication is enabled.
	// Custom roles must be created with login=true.
	// Default: alwayson_sql
	// +optional
	AuthUser *string `json:"auth_user,omitempty" cass-config:"dse:*:auth_user"`

	// The maximum number of errors that can occur during AlwaysOn SQL service runner thread runs
	// before stopping the service. A service stop requires a manual restart.
	// Default: 10
	// +optional
	RunnerMaxErrors *int `json:"runner_max_errors,omitempty" cass-config:"dse:*:runner_max_errors"`

	// The time interval to update heartbeat of AlwaysOn SQL. If heartbeat is not updated for more
	// than three times the interval, AlwaysOn SQL automatically restarts.
	// Default: 30
	// +optional
	HeartbeatUpdateIntervalSeconds *int `json:"heartbeat_update_interval_seconds,omitempty" cass-config:"dse:*:heartbeat_update_interval_seconds"`
}

type DsefsOptions struct {

	// Enables DSEFS.
	// - true: Enables DSEFS on this node, regardless of the workload.
	// - false: Disables DSEFS on this node, regardless of the workload.
	// If blank or commented out,  DSEFS starts only if the node is configured to run analytics
	// workloads.
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The keyspace where the DSEFS metadata is stored. You can optionally configure multiple DSEFS
	// file systems within a single datacenter by specifying different keyspace names for each
	// cluster.
	// Default: dsefs
	// +optional
	KeyspaceName *string `json:"keyspace_name,omitempty" cass-config:"dse:*:keyspace_name"`

	// The local directory for storing the local node metadata, including the node identifier. The
	// volume of data stored in this directory is nominal and does not require configuration for
	// throughput, latency, or capacity. This directory must not be shared by DSEFS nodes.
	// Default: /var/lib/dsefs
	// +optional
	// TODO mountable directory
	WorkDir *string `json:"work_dir,omitempty" cass-config:"dse:*:work_dir"`

	// The public port on which DSEFS listens for clients.
	// Note: DataStax recommends that all nodes in the cluster have the same value. Firewalls must
	// open this port to trusted clients. The service on this port is bound to the
	// native_transport_address.
	// Default: 5598
	// +optional
	PublicPort *int `json:"public_port,omitempty" cass-config:"dse:*:public_port"`

	// The private port for DSEFS internode communication.
	// CAUTION: Do not open this port to firewalls; this private port must be not visible from
	// outside the cluster.
	// Default: 5599
	// +optional
	PrivatePort *int `json:"private_port,omitempty" cass-config:"dse:*:private_port"`

	// One or more data locations where the DSEFS data is stored.
	// +optional
	DataDirectories []DsefsDataDirectory `json:"data_directories,omitempty" cass-config:"dse:*:data_directories;recurse"`

	// Wait time in milliseconds before the DSEFS server times out while waiting for services to
	// bootstrap.
	// Default: 60000
	// +optional
	ServiceStartupTimeoutMs *int `json:"service_startup_timeout_ms,omitempty" cass-config:"dse:*:service_startup_timeout_ms"`

	// Wait time in milliseconds before the DSEFS server times out while waiting for services to
	// close.
	// Default: 60000
	// +optional
	ServiceCloseTimeoutMs *int `json:"service_close_timeout_ms,omitempty" cass-config:"dse:*:service_close_timeout_ms"`

	// Wait time in milliseconds that the DSEFS server waits during shutdown before closing all
	// pending connections.
	// Default: 2147483647
	// +optional
	ServerCloseTimeoutMs *int `json:"server_close_timeout_ms,omitempty" cass-config:"dse:*:server_close_timeout_ms"`

	// The maximum accepted size of a compression frame defined during file upload.
	// Default: 1048576
	// +optional
	CompressionFrameMaxSize *int `json:"compression_frame_max_size,omitempty" cass-config:"dse:*:compression_frame_max_size"`

	// Maximum number of elements in a single DSEFS Server query cache.
	// Default: 2048
	// +optional
	QueryCacheSize *int `json:"query_cache_size,omitempty" cass-config:"dse:*:query_cache_size"`

	// The time to retain the DSEFS Server query cache element in cache. The cache element expires
	// when this time is exceeded.
	// Default: 2000
	// +optional
	QueryCacheExpireAfterMs *int `json:"query_cache_expire_after_ms,omitempty" cass-config:"dse:*:query_cache_expire_after_ms"`

	// Configures DSEFS gossip rounds.
	// +optional
	GossipOptions *DsefsGossipOptions `json:"gossip_options,omitempty" cass-config:"dse:*:gossip_options;recurse"`

	// Configures DSEFS rest times.
	// +optional
	RestOptions *DsefsRestOptions `json:"rest_options,omitempty" cass-config:"dse:*:rest_options;recurse"`

	// Configures DSEFS transaction times.
	// +optional
	TransactionOptions *DsefsTransactionOptions `json:"transaction_options,omitempty" cass-config:"dse:*:transaction_options;recurse"`

	// Controls how much additional data can be placed on the local coordinator before the local
	// node overflows to the other nodes. The trade-off is between data locality of writes and
	// balancing the cluster. A local node is preferred for a new block allocation, if:
	// used_size_on_the_local_node < average_used_size_per_node x overflow_factor + overflow_margin.
	// +optional
	BlockAllocatorOptions *DsefsBlockAllocatorOptions `json:"block_allocator_options,omitempty" cass-config:"dse:*:block_allocator_options;recurse"`
}

type DsefsDataDirectory struct {

	// Mandatory attribute to identify the set of directories. DataStax recommends segregating
	// these data directories on physical devices that are different from the devices that are used
	// for DataStax Enterprise. Using multiple directories on JBOD improves performance and capacity.
	// Default: /var/lib/dsefs/data
	// +optional
	// TODO mountable directory
	Dir *string `json:"dir,omitempty" cass-config:"dse:*:dir"`

	// Weighting factor for this location. Determines how much data to place in this directory,
	// relative to other directories in the cluster. This soft constraint determines how DSEFS
	// distributes the data. For example, a directory with a value of 3.0 receives about three times
	// more data than a directory with a value of 1.0.
	// Default: 1.0
	// +optional
	StorageWeight *resource.Quantity `json:"storage_weight,omitempty" cass-config:"dse:*:storage_weight"`

	// min_free_space
	// The reserved space, in bytes, to not use for storing file data blocks.
	// Default: 268435456
	// +optional
	MinFreeSpace *resource.Quantity `json:"min_free_space,omitempty" cass-config:"dse:*:min_free_space"`
}

type DsefsGossipOptions struct {

	// round_delay_ms
	// The delay in milliseconds between gossip rounds.
	// Default: 2000
	// +optional
	RoundDelayMs *int `json:"round_delay_ms,omitempty" cass-config:"dse:*:round_delay_ms"`

	// The delay in milliseconds between registering the location and reading back all other
	// locations from the database.
	// Default: 5000
	// +optional
	StartupDelayMs *int `json:"startup_delay_ms,omitempty" cass-config:"dse:*:startup_delay_ms"`

	// The delay time in milliseconds between announcing shutdown and shutting down the node.
	// Default: 30000
	// +optional
	ShutdownDelayMs *int `json:"shutdown_delay_ms,omitempty" cass-config:"dse:*:shutdown_delay_ms"`
}

type DsefsRestOptions struct {

	// The time in milliseconds that the client waits for a response that corresponds to a given request.
	// Default: 330000
	// +optional
	RequestTimeoutMs *int `json:"request_timeout_ms,omitempty" cass-config:"dse:*:request_timeout_ms"`

	// The time in milliseconds that the client waits to establish a new connection.
	// Default: 55000
	// +optional
	ConnectionOpenTimeoutMs *int `json:"connection_open_timeout_ms,omitempty" cass-config:"dse:*:connection_open_timeout_ms"`

	// The time in milliseconds that the client waits for pending transfer to complete before
	// closing a connection.
	// Default: 60000
	// +optional
	ClientCloseTimeoutMs *int `json:"client_close_timeout_ms,omitempty" cass-config:"dse:*:client_close_timeout_ms"`

	// The time in milliseconds to wait for the server rest call to complete.
	// Default: 300000
	// +optional
	ServerRequestTimeoutMs *int `json:"server_request_timeout_ms,omitempty" cass-config:"dse:*:server_request_timeout_ms"`

	// The time in milliseconds for RestClient to wait before closing an idle connection. If
	// RestClient does not close connection after timeout, the connection is closed after 2 x this
	// wait time.
	// - time: Wait time to close idle connection.
	// - 0: Disable closing idle connections.
	// Default: 60000
	// +optional
	IdleConnectionTimeoutMs *int `json:"idle_connection_timeout_ms,omitempty" cass-config:"dse:*:idle_connection_timeout_ms"`

	// Wait time in milliseconds before closing idle internode connection. The internode connections
	// are primarily used to exchange data during replication. Do not set lower than the default
	// value for heavily utilized DSEFS clusters.
	// Default: 0
	// +optional
	InternodeIdleConnectionTimeoutMs *int `json:"internode_idle_connection_timeout_ms,omitempty" cass-config:"dse:*:internode_idle_connection_timeout_ms"`

	// Maximum number of connections to a given host per single CPU core. DSEFS keeps a connection
	// pool for each CPU core.
	// Default: 8
	// +optional
	CoreMaxConcurrentConnectionsPerHost *int `json:"core_max_concurrent_connections_per_host,omitempty" cass-config:"dse:*:core_max_concurrent_connections_per_host"`
}

type DsefsTransactionOptions struct {

	// Transaction run time in milliseconds before the transaction is considered for timeout and
	// rollback.
	// Default: 3000
	// +optional
	TransactionTimeoutMs *int `json:"transaction_timeout_ms,omitempty" cass-config:"dse:*:transaction_timeout_ms"`

	// Wait time in milliseconds before retrying a transaction that was ended due to a conflict.
	// Default: 200
	// +optional
	ConflictRetryDelayMs *int `json:"conflict_retry_delay_ms,omitempty" cass-config:"dse:*:conflict_retry_delay_ms"`

	// The number of times to retry a transaction before giving up.
	// Default: 40
	// +optional
	ConflictRetryCount *int `json:"conflict_retry_count,omitempty" cass-config:"dse:*:conflict_retry_count"`

	// Wait time in milliseconds before retrying a failed transaction payload execution.
	// Default: 1000
	// +optional
	ExecutionRetryDelayMs *int `json:"execution_retry_delay_ms,omitempty" cass-config:"dse:*:execution_retry_delay_ms"`

	// The number of payload execution retries before signaling the error to the application.
	// Default: 3
	// +optional
	ExecutionRetryCount *int `json:"execution_retry_count,omitempty" cass-config:"dse:*:execution_retry_count"`
}

type DsefsBlockAllocatorOptions struct {

	// - margin_size: Overflow margin size in megabytes.
	// - 0: Disable block allocation overflow
	// Default: 1024
	// +optional
	OverflowMarginMb *int `json:"overflow_margin_mb,omitempty" cass-config:"dse:*:overflow_margin_mb"`

	// - factor: Overflow factor on an exponential scale.
	// - 1.0: Disable block allocation overflow
	// Default: 1.05
	// +optional
	OverflowFactor *resource.Quantity `json:"overflow_factor,omitempty" cass-config:"dse:*:overflow_factor"`
}

type InsightsOptions struct {

	// Directory to store collected metrics.
	// When data_dir is not explicitly set, the insights_data directory is stored in the same parent
	// directory as the commitlog_directory as defined in cassandra.yaml. If the commitlog_directory
	// uses the package default of /var/lib/cassandra/commitlog, data_dir will default to
	// /var/lib/cassandra/insights_data.
	// Default: /var/lib/cassandra/insights_data
	// +optional
	// TODO mountable directory
	DataDir *string `json:"data_dir,omitempty" cass-config:"dse:*:data_dir"`

	// Directory to store logs for collected metrics. The log file is dse-collectd.log. The file
	// with the collectd PID is dse-collectd.pid.
	// Default: /var/log/cassandra/
	// +optional
	// TODO mountable directory
	LogDir *string `json:"log_dir,omitempty" cass-config:"dse:*:log_dir"`
}

type AuditLoggingOptions struct {

	// Enables database activity auditing.
	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The logger to use for recording events:
	// - SLF4JAuditWriter: Capture events in a log file.
	// - CassandraAuditWriter: Capture events in the dse_audit.audit_log table.
	// Tip: Configure logging level, sensitive data masking, and log file name/location in the
	// logback.xml file.
	// Default: SLF4JAuditWriter
	// +optional
	// +kubebuilder:validation:Enum=SLF4JAuditWriter;CassandraAuditWriter
	Logger *string `json:"logger,omitempty" cass-config:"dse:*:logger"`

	// Comma-separated list of event categories that are captured.
	// - QUERY: Data retrieval events.
	// - DML: (Data manipulation language) Data change events.
	// - DDL: (Data definition language) Database schema change events.
	// - DCL: (Data change language) Role and permission management events.
	// - AUTH: (Authentication) Login and authorization related events.
	// - ERROR: Failed requests.
	// - UNKNOWN: Events where the category and type are both UNKNOWN.
	// Event categories that are not listed are not captured.
	// WARNING: Use either included_categories or excluded_categories but not both. When specifying
	// included categories leave excluded_categories blank or commented out.
	// Default: none (include all categories)
	// +optional
	IncludedCategories *string `json:"included_categories,omitempty" cass-config:"dse:*:included_categories"`

	// Comma-separated list of categories to ignore, where the categories are:
	// - QUERY: Data retrieval events.
	// - DML: (Data manipulation language) Data change events.
	// - DDL: (Data definition language) Database schema change events.
	// - DCL: (Data change language) Role and permission management events.
	// - AUTH: (Authentication) Login and authorization related events.
	// - ERROR: Failed requests.
	// - UNKNOWN: Events where the category and type are both UNKNOWN.
	// Events in all other categories are logged.
	// WARNING: Use either included_categories or excluded_categories but not both.
	// Default: exclude no categories
	// +optional
	ExcludedCategories *string `json:"excluded_categories,omitempty" cass-config:"dse:*:excluded_categories"`

	// Comma-separated list of keyspaces for which events are logged. You can also use a regular
	// expression to filter on keyspace name.
	// WARNING: DSE supports using either included_keyspaces or excluded_keyspaces but not both.
	// Default: include all keyspaces
	// +optional
	IncludedKeyspaces *string `json:"included_keyspaces,omitempty" cass-config:"dse:*:included_keyspaces"`

	// Comma-separated list of keyspaces to exclude. You can also use a regular expression to filter
	// on keyspace name.
	// Default: exclude no keyspaces
	// +optional
	ExcludedKeyspaces *string `json:"excluded_keyspaces,omitempty" cass-config:"dse:*:excluded_keyspaces"`

	// Comma-separated list of the roles for which events are logged.
	// WARNING: DSE supports using either included_roles or excluded_roles but not both.
	// Default: include all roles
	// +optional
	IncludedRoles *string `json:"included_roles,omitempty" cass-config:"dse:*:included_roles"`

	// excluded_roles
	// The roles for which events are not logged. Specify a comma separated list role names.
	// Default: exclude no roles
	// +optional
	ExcludedRoles *string `json:"excluded_roles,omitempty" cass-config:"dse:*:excluded_roles"`

	// The number of hours to retain audit events by supporting loggers for the CassandraAuditWriter.
	// - hours: The number of hours to retain audit events.
	// - 0: Retain events forever.
	// Default: 0
	// +optional
	AuditLoggingRetentionTimeHours *int `json:"retention_time,omitempty" cass-config:"dse:*:retention_time"`

	// Options for logger CassandraAuditWriter.
	// +optional
	CassandraAuditWriterOptions *CassandraAuditWriterOptions `json:"cassandra_audit_writer_options,omitempty" cass-config:"dse:*:cassandra_audit_writer_options;recurse"`
}

type CassandraAuditWriterOptions struct {

	// The mode the writer runs in.
	// - sync: A query is not executed until the audit event is successfully written.
	// - async: Audit events are queued for writing to the audit table, but are not necessarily
	//   logged before the query executes. A pool of writer threads consumes the audit events from
	//   the queue, and writes them to the audit table in batch queries.
	// Important: While async substantially improves performance under load, if there is a failure
	// between when a query is executed, and its audit event is written to the table, the audit
	// table might be missing entries for queries that were executed.
	// Default: sync
	// +optional
	// +kubebuilder:validation:Enum=sync;async
	Mode *string `json:"mode,omitempty" cass-config:"dse:*:mode"`

	// Available only when mode: async. Must be greater than 0.
	// The maximum number of events the writer dequeues before writing them out to the table. If
	// warnings in the logs reveal that batches are too large, decrease this value or increase the
	// value of batch_size_warn_threshold_in_kb in cassandra.yaml.
	// Default: 50
	// +optional
	BatchSize *int `json:"batch_size,omitempty" cass-config:"dse:*:batch_size"`

	// Available only when mode: async.
	// The maximum amount of time in milliseconds before an event is removed from the queue by a
	// writer before being written out. This flush time prevents events from waiting too long before
	// being written to the table when there are not a lot of queries happening.
	// Default: 500
	// +optional
	FlushTimeMs *int `json:"flush_time,omitempty" cass-config:"dse:*:flush_time"`

	// The size of the queue feeding the asynchronous audit log writer threads.
	// - Number of events: When there are more events being produced than the writers can write out,
	//   the queue fills up, and newer queries are blocked until there is space on the queue.
	// - When set to zero, the queue size is unbounded, which can lead to resource exhaustion under
	//   heavy query load.
	// Default: 30000
	// +optional
	QueueSize *int `json:"queue_size,omitempty" cass-config:"dse:*:queue_size"`

	// The consistency level that is used to write audit events.
	// Default: QUORUM
	// +optional
	// +kubebuilder:validation:Enum=ANY;ONE;TWO;THREE;QUORUM;ALL;LOCAL_QUORUM;EACH_QUORUM;SERIAL;LOCAL_SERIAL;LOCAL_ONE
	WriteConsistency *string `json:"write_consistency,omitempty" cass-config:"dse:*:write_consistency"`

	// dropped_event_log
	// The directory to store the log file that reports dropped events.
	// Default: /var/log/cassandra/dropped_audit_events.log
	// +optional
	// TODO mountable directory
	DroppedEventLog *string `json:"dropped_event_log,omitempty" cass-config:"dse:*:dropped_event_log"`

	// The time interval in milliseconds between changing nodes to spread audit log information
	// across multiple nodes. For example, to change the target node every 12 hours, specify
	// 43200000 milliseconds.
	// Default: 3600000 (1 hour)
	// +optional
	DayPartitionMillis *int `json:"day_partition_millis,omitempty" cass-config:"dse:*:day_partition_millis"`
}

type TieredStorageOptions struct {

	// +optional
	Tiers []StorageTier `json:"tiers,omitempty" cass-config:"dse:*:tiers;recurse"`

	// +optional
	LocalOptions map[string]string `json:"local_options,omitempty" cass-config:"dse:*:local_options"`
}

type StorageTier struct {

	// +optional
	Paths []string `json:"paths,omitempty" cass-config:"dse:*:paths"`
}

type AdvancedReplicationOptions struct {

	// Enables an edge node to collect data in the replication log.
	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// Enables encryption of driver passwords. See Encrypting configuration file properties.
	// Default: false
	// +optional
	ConfDriverPasswordEncryptionEnabled *bool `json:"conf_driver_password_encryption_enabled,omitempty" cass-config:"dse:*:conf_driver_password_encryption_enabled"`

	// The directory for storing advanced replication CDC logs. The replication_logs directory will
	// be created in the specified directory.
	// Default: /var/lib/cassandra/advrep
	// +optional
	// TODO mountable directory
	AdvancedReplicationDirectory *string `json:"advanced_replication_directory,omitempty" cass-config:"dse:*:advanced_replication_directory"`

	// The base path to prepend to paths in the Advanced Replication configuration locations,
	// including locations to SSL keystore, SSL truststore, and so on.
	// Default: /base/path/to/advrep/security/files/
	// +optional
	// TODO mountable directory
	SecurityBasePath *string `json:"security_base_path,omitempty" cass-config:"dse:*:security_base_path"`
}

type InternodeMessagingOptions struct {

	// The mandatory port for the internode messaging service.
	// Default: 8609
	// +optional
	Port *int `json:"port,omitempty" cass-config:"dse:*:port"`

	// Maximum message frame length.
	// Default: 256
	// +optional
	FrameLengthInMb *int `json:"frame_length_in_mb,omitempty" cass-config:"dse:*:frame_length_in_mb"`

	// server_acceptor_threads
	// The number of server acceptor threads.
	// Default: The number of available processors
	// +optional
	ServerAcceptorThreads *int `json:"server_acceptor_threads,omitempty" cass-config:"dse:*:server_acceptor_threads"`

	// The number of server worker threads.
	// Default: The default is the number of available processors x 8
	// +optional
	ServerWorkerThreads *int `json:"server_worker_threads,omitempty" cass-config:"dse:*:server_worker_threads"`

	// The maximum number of client connections.
	// Default: 100
	// +optional
	ClientMaxConnections *int `json:"client_max_connections,omitempty" cass-config:"dse:*:client_max_connections"`

	// The number of client worker threads.
	// Default: The default is the number of available processors x 8
	// +optional
	ClientWorkerThreads *int `json:"client_worker_threads,omitempty" cass-config:"dse:*:client_worker_threads"`

	// Timeout for communication handshake process.
	// Default: 10
	// +optional
	HandshakeTimeoutSeconds *int `json:"handshake_timeout_seconds,omitempty" cass-config:"dse:*:handshake_timeout_seconds"`

	// Timeout for non-query search requests like core creation and distributed deletes.
	// Default: 60
	// +optional
	ClientRequestTimeoutSeconds *int `json:"client_request_timeout_seconds,omitempty" cass-config:"dse:*:client_request_timeout_seconds"`
}

type GremlinServerOptions struct {

	// The available communications port for Gremlin Server.
	// Default: 8182
	// +optional
	Port *int `json:"port,omitempty" cass-config:"dse:*:port"`

	// The number of worker threads that handle non-blocking read and write (requests and responses)
	// on the Gremlin Server channel, including routing requests to the right server operations,
	// handling scheduled jobs on the server, and writing serialized responses back to the client.
	// Default: 2
	// +optional
	ThreadPoolWorker *int `json:"threadPoolWorker,omitempty" cass-config:"dse:*:threadPoolWorker"`

	// This pool represents the workers available to handle blocking operations in Gremlin Server.
	// - 0: the value of the JVM property cassandra.available_processors, if that property is set
	// - positive number: The number of Gremlin threads available to execute actual scripts in a
	//   ScriptEngine.
	// Default: the value of Runtime.getRuntime().availableProcessors()
	// +optional
	GremlinPool *int `json:"gremlinPool,omitempty" cass-config:"dse:*:gremlinPool"`

	// Configures gremlin server script engines.
	// +optional
	ScriptEngines *GremlinScriptEngine `json:"scriptEngines,omitempty" cass-config:"dse:*:scriptEngines;recurse"`
}

type GremlinScriptEngine struct {

	// Configures for gremlin-groovy scripts.
	// +optional
	GremlinGroovy *GremlinGroovy `json:"gremlin-groovy,omitempty" cass-config:"dse:*:gremlin-groovy;recurse"`
}

type GremlinGroovy struct {

	// +optional
	Config *GremlinGroovyConfig `json:"config,omitempty" cass-config:"dse:*:config;recurse"`
}

type GremlinGroovyConfig struct {

	// Enables gremlin groovy sandbox.
	// Default: true
	// +optional
	SandboxEnabled *bool `json:"sandbox_enabled,omitempty" cass-config:"dse:*:sandbox_enabled"`

	// Configures sandbox rules.
	// +optional
	SandboxRules *GremlinGroovySandboxRules `json:"sandbox_rules,omitempty" cass-config:"dse:*:sandbox_rules;recurse"`
}

type GremlinGroovySandboxRules struct {

	// List of packages, one package per line, to whitelist.
	// Default: []
	// +optional
	WhitelistPackages []string `json:"whitelist_packages,omitempty" cass-config:"dse:*:whitelist_packages"`

	// List of fully qualified types, one type per line, to whitelist.
	// Default: []
	// +optional
	WhitelistTypes []string `json:"whitelist_types,omitempty" cass-config:"dse:*:whitelist_types"`

	// List of super classes, one class per line, to whitelist.
	// Default: []
	// +optional
	WhitelistSupers []string `json:"whitelist_supers,omitempty" cass-config:"dse:*:whitelist_supers"`

	// List of packages, one package per line, to blacklist.
	// Default: []
	// +optional
	// FIXME not implemented yet in config-builder
	BlacklistPackages []string `json:"blacklist_packages,omitempty" cass-config:"dse:*:blacklist_packages"`

	// List of super classes, one class per line, to blacklist.
	// Default: []
	// +optional
	// FIXME not implemented yet in config-builder
	BlacklistSupers []string `json:"blacklist_supers,omitempty" cass-config:"dse:*:blacklist_supers"`
}

type GraphOptions struct {

	// Maximum time to wait for an OLAP analytic (Spark) traversal to evaluate.
	// Default: 10080 (168 hours)
	// +optional
	AnalyticEvaluationTimeoutInMinutes *int `json:"analytic_evaluation_timeout_in_minutes,omitempty" cass-config:"dse:*:analytic_evaluation_timeout_in_minutes"`

	// Maximum time to wait for an OLTP real-time traversal to evaluate.
	// Default: 30
	// +optional
	RealtimeEvaluationTimeoutInSeconds *int `json:"realtime_evaluation_timeout_in_seconds,omitempty" cass-config:"dse:*:realtime_evaluation_timeout_in_seconds"`

	// Maximum time to wait for the database to agree on schema versions before timing out.
	// Default: 10000
	// +optional
	SchemaAgreementTimeoutInMs *int `json:"schema_agreement_timeout_in_ms,omitempty" cass-config:"dse:*:schema_agreement_timeout_in_ms"`

	// Maximum time to wait for a graph system-based request to execute, like creating a new graph.
	// Default: 180 (3 minutes)
	// +optional
	SystemEvaluationTimeoutInSeconds *int `json:"system_evaluation_timeout_in_seconds,omitempty" cass-config:"dse:*:system_evaluation_timeout_in_seconds"`

	// The amount of ram to allocate to each graph's adjacency (edge and property) cache.
	// Default: 128
	// +optional
	AdjacencyCacheSizeInMb *int `json:"adjacency_cache_size_in_mb,omitempty" cass-config:"dse:*:adjacency_cache_size_in_mb"`

	// The amount of ram to allocate to the index cache.
	// Default: 128
	// +optional
	IndexCacheSizeInMb *int `json:"index_cache_size_in_mb,omitempty" cass-config:"dse:*:index_cache_size_in_mb"`

	// The maximum number of parameters that can be passed on a graph query request for TinkerPop
	// drivers and drivers using the Cassandra native protocol.
	// Default: 16
	// +optional
	MaxQueryParams *int `json:"max_query_params,omitempty" cass-config:"dse:*:max_query_params"`

	// Gremlin Server configuration (DSE Graph).
	// +optional
	GremlinServerOptions *GremlinServerOptions `json:"gremlin_server,omitempty" cass-config:"dse:*:gremlin_server;recurse"`
}

type AnalyticsOptions struct {

	// The length of a shared secret used to authenticate Spark components and encrypt the
	// connections between them. This value is not the strength of the cipher for encrypting
	// connections.
	// Default: 256
	// +optional
	SparkSharedSecretBitLength *int `json:"spark_shared_secret_bit_length,omitempty" cass-config:"dse:*:spark_shared_secret_bit_length"`

	// When DSE authentication is enabled with authentication_options, Spark security is enabled
	// regardless of this setting.
	// Default: false
	// +optional
	SparkSecurityEnabled *bool `json:"spark_security_enabled,omitempty" cass-config:"dse:*:spark_security_enabled"`

	// When DSE authentication is enabled with authentication_options, Spark security encryption is
	// enabled regardless of this setting.
	// Tip: Configure encryption between the Spark processes and DSE with client-to-node encryption
	// in cassandra.yaml.
	// Default: false
	// +optional
	SparkSecurityEncryptionEnabled *bool `json:"spark_security_encryption_enabled,omitempty" cass-config:"dse:*:spark_security_encryption_enabled"`

	// Time interval in milliseconds between subsequent retries by the Spark plugin for Spark Master and Worker readiness to start.
	// Default: 1000
	// +optional
	SparkDaemonReadinessAssertionIntervalMs *int `json:"spark_daemon_readiness_assertion_interval,omitempty" cass-config:"dse:*:spark_daemon_readiness_assertion_interval"`

	// Controls the physical resources used by Spark applications on this node.
	// +optional
	ResourceManagerOptions *SparkResourceManagerOptions `json:"resource_manager_options,omitempty" cass-config:"dse:*:resource_manager_options;recurse"`

	// Configures encryption for Spark Master and Spark Worker UIs. These options apply only to
	// Spark daemon UIs, and do not apply to user applications even when the user applications are
	// run in cluster mode.
	// +optional
	SparkUiOptions *SparkUiOptions `json:"spark_ui_options,omitempty" cass-config:"dse:*:spark_ui_options;recurse"`

	// Configures how Spark driver and executor processes are created and managed.
	// +optional
	SparkProcessRunner *SparkProcessRunner `json:"spark_process_runner,omitempty" cass-config:"dse:*:spark_process_runner;recurse"`
}

type SparkResourceManagerOptions struct {

	// Configures the amount of system resources that are made available to the Spark Worker.
	// +optional
	WorkerOptions *SparkWorkerOptions `json:"worker_options,omitempty" cass-config:"dse:*:worker_options;recurse"`
}

type SparkWorkerOptions struct {

	// The number of total system cores available to Spark.
	// Note: The SPARK_WORKER_TOTAL_CORES environment variables takes precedence over this setting.
	// The lowest value that you can assign to Spark Worker cores is 1 core. If the results are
	// lower, no exception is thrown and the values are automatically limited.
	// Note: Setting cores_total or a workpool's cores to 1.0 is a decimal value, meaning 100% of
	// the available cores will be reserved. Setting cores_total or cores to 1 (no decimal point) is
	// an explicit value, and one core will be reserved.
	// Default: 0.7
	// +optional
	CoresTotal *resource.Quantity `json:"cores_total,omitempty" cass-config:"dse:*:cores_total"`

	// The amount of total system memory available to Spark.
	// - absolute value - Use standard suffixes like M for megabyte and G for gigabyte. For example,
	//   12G.
	// - decimal value - Maximum fraction of system memory to give all executors for all
	//   applications running on a particular node. For example, 0.8.
	// When the value is expressed as a decimal, the available resources are calculated in the
	// following way:
	// Spark Worker memory = memory_total x (total system memory - memory assigned to DataStax
	// Enterprise)
	// The lowest values that you can assign to Spark Worker memory is 64 MB. If the results are
	// lower, no exception is thrown and the values are automatically limited.
	// Note: The SPARK_WORKER_TOTAL_MEMORY environment variables takes precedence over this setting.
	// Default: 0.6
	// +optional
	MemoryTotal *resource.Quantity `json:"memory_total,omitempty" cass-config:"dse:*:memory_total"`

	// A collection of named workpools that can use a portion of the total resources defined under
	// worker_options.
	// A default workpool named default is used if no workpools are defined in this section. If
	// workpools are defined, the resources allocated to the workpools are taken from the total
	// amount, with the remaining resources available to the default workpool.
	// The total amount of resources defined in the workpools section must not exceed the resources
	// available to Spark in worker_options.
	// +optional
	Workpools []SparkWorkpool `json:"workpools,omitempty" cass-config:"dse:*:workpools;recurse"`
}

type SparkWorkpool struct {

	// The name of the workpool. A workpool named alwayson_sql is created by default for AlwaysOn
	// SQL. By default, the alwayson_sql workpool is configured to use 25% of the resources
	// available to Spark.
	// Default: alwayson_sql
	// +optional
	Name *string `json:"name,omitempty" cass-config:"dse:*:name"`

	// The number of system cores to use in this workpool expressed as an absolute value or a
	// decimal value. This option follows the same rules as cores_total.
	// +optional
	Cores *resource.Quantity `json:"cores,omitempty" cass-config:"dse:*:cores"`

	// The amount of memory to use in this workpool expressed as either an absolute value or a
	// decimal value. This option follows the same rules as memory_total.
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty" cass-config:"dse:*:memory"`
}

type SparkUiOptions struct {

	// The source for SSL settings.
	// - inherit: Inherit the SSL settings from the client_encryption_options in cassandra.yaml.
	// - custom: Use the following encryption_options in dse.yaml.
	// Default: inherit
	// +optional
	// +kubebuilder:validation:Enum=inherit;custom
	Encryption *string `json:"encryption,omitempty" cass-config:"dse:*:encryption"`

	// When encryption = custom, configures encryption for HTTPS of Spark Master and Worker UI.
	// +optional
	EncryptionOptions *SparkUiEncryptionOptions `json:"encryption_options,omitempty" cass-config:"dse:*:encryption_options;recurse"`
}

type SparkUiEncryptionOptions struct {

	// Enables Spark encryption for Spark client-to-Spark cluster and Spark internode communication.
	// Default: false
	// +optional
	Enabled *bool `json:"enabled,omitempty" cass-config:"dse:*:enabled"`

	// The keystore for Spark encryption keys.
	// The relative filepath is the base Spark configuration directory that is defined by the
	// SPARK_CONF_DIR environment variable. The default Spark configuration directory is
	// resources/spark/conf.
	// Default: resources/dse/conf/.ui-keystore
	// +optional
	// TODO mountable directory
	Keystore *string `json:"keystore,omitempty" cass-config:"dse:*:keystore"`

	// The password to access the keystore.
	// Default: cassandra
	// +optional
	KeystorePassword *string `json:"keystore_password,omitempty" cass-config:"dse:*:keystore_password"`

	// Enables custom truststore for client authentication.
	// Default: false
	// +optional
	RequireClientAuth *bool `json:"require_client_auth,omitempty" cass-config:"dse:*:require_client_auth"`

	// The filepath to the truststore for Spark encryption keys if require_client_auth: true.
	// The relative filepath is the base Spark configuration directory that is defined by the SPARK_CONF_DIR environment variable. The default Spark configuration directory is resources/spark/conf.
	// Default: resources/dse/conf/.ui-truststore
	// +optional
	// TODO mountable directory
	Truststore *string `json:"truststore,omitempty" cass-config:"dse:*:truststore"`

	// The password to access the truststore.
	// Default: cassandra
	// +optional
	TruststorePassword *string `json:"truststore_password,omitempty" cass-config:"dse:*:truststore_password"`

	// The Transport Layer Security (TLS) authentication protocol. The TLS protocol must be
	// supported by JVM and Spark. TLS 1.2 is the most common JVM default.
	// Default: JVM default
	// +optional
	Protocol *string `json:"protocol,omitempty" cass-config:"dse:*:protocol"`

	// The key manager algorithm.
	// Default: SunX509
	// +optional
	Algorithm *string `json:"algorithm,omitempty" cass-config:"dse:*:algorithm"`

	// Valid types are JKS, JCEKS, PKCS11, and PKCS12. For file-based keystores, use PKCS12.
	// Default: JKS
	// +optional
	// +kubebuilder:validation:Enum=JKS;JCEKS;PKCS11;PKCS12
	KeystoreType *string `json:"keystore_type,omitempty" cass-config:"dse:*:keystore_type"`

	// Valid types are JKS, JCEKS, and PKCS12.
	// Default: JKS
	// +optional
	// +kubebuilder:validation:Enum=JKS;JCEKS;PKCS12
	TruststoreType *string `json:"truststore_type,omitempty" cass-config:"dse:*:truststore_type"`

	// A comma-separated list of cipher suites for Spark encryption. Enclose the list in square brackets.
	// Default: [TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_DHE_RSA_WITH_AES_128_CBC_SHA,TLS_DHE_RSA_WITH_AES_256_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA]
	// +optional
	CipherSuites *string `json:"cipher_suites,omitempty" cass-config:"dse:*:cipher_suites"`
}

type SparkProcessRunner struct {

	// - default: Use the default runner type.
	// - run_as: Spark applications run as a different OS user than the DSE service user.
	// +optional
	// +kubebuilder:validation:Enum=default;run_as
	RunnerType *string `json:"runner_type,omitempty" cass-config:"dse:*:runner_type"`

	// When runner_type: run_as, Spark applications run as a different OS user than the DSE service
	// user.
	// +optional
	RunAsRunnerOptions *SparkRunAsRunnerOptions `json:"run_as_runner_options,omitempty" cass-config:"dse:*:run_as_runner_options;recurse"`
}

type SparkRunAsRunnerOptions struct {

	// List of slot users to separate Spark processes users from the DSE service user.
	// Default: slot1, slot2
	// +optional
	UserSlots []string `json:"user_slots,omitempty" cass-config:"dse:*:user_slots"`
}

type IndexOptions struct {

	// +optional
	// FIXME not present in the official documentation
	SegmentWriteBufferSpaceMb *int `json:"segment_write_buffer_space_mb,omitempty" cass-config:"dse:*:segment_write_buffer_space_mb"`

	// +optional
	// FIXME not present in the official documentation
	ZerocopyUsedThreshold *resource.Quantity `json:"zerocopy_used_threshold,omitempty" cass-config:"dse:*:zerocopy_used_threshold"`
}
