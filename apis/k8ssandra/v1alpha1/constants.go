package v1alpha1

const (
	ResourceHashAnnotation = "k8ssandra.io/resource-hash"

	// PerNodeConfigHashAnnotation is the annotation used to store the hash of the per-node
	// ConfigMap into the PodTemplateSpec of the CassandraDatacenter resource. By storing the
	// ConfigMap hash, the PodTemplateSpec changes when the ConfigMap changes, thus allowing the
	// changes to the ConfigMap to be properly detected and applied.
	PerNodeConfigHashAnnotation = "k8ssandra.io/per-node-config-hash"

	// InitialSystemReplicationAnnotation provides the initial replication of system keyspaces
	// (system_auth, system_distributed, system_traces) encoded as JSON. This annotation
	// is set on a K8ssandraCluster when it is first created. The value does not change
	// regardless of whether the replication of the system keyspaces changes.
	InitialSystemReplicationAnnotation = "k8ssandra.io/initial-system-replication"

	// DcReplicationAnnotation tells the operator the replication settings to apply to user
	// keyspaces when adding a DC to an existing cluster. The value should be serialized
	// JSON, e.g., {"dc2": {"ks1": 3, "ks2": 3}}. All user keyspaces must be specified;
	// otherwise, reconciliation will fail with a validation error. If you do not want to
	// replicate a particular keyspace, specify a value of 0. Replication settings can be
	// specified for multiple DCs; however, existing DCs won't be modified, and only the DC
	// currently being added will be updated. Specifying multiple DCs can be useful though
	// if you add multiple DCs to the cluster at once (Note that the CassandraDatacenters
	// are still deployed serially).
	DcReplicationAnnotation = "k8ssandra.io/dc-replication"

	// RebuildSourceDcAnnotation tells the operation the DC from which to stream when
	// rebuilding a DC. If not set the operator will choose the first DC. The value for
	// this annotation must specify the name of a CassandraDatacenter whose Ready
	// condition is true.
	RebuildSourceDcAnnotation = "k8ssandra.io/rebuild-src-dc"

	RebuildDcAnnotation = "k8ssandra.io/rebuild-dc"

	NameLabel      = "app.kubernetes.io/name"
	NameLabelValue = "k8ssandra-operator"

	ManagedByLabel = "app.kubernetes.io/managed-by"

	ComponentLabel               = "app.kubernetes.io/component"
	ComponentLabelValueCassandra = "cassandra"
	ComponentLabelValueStargate  = "stargate"
	ComponentLabelValueReaper    = "reaper"
	ComponentLabelTelemetry      = "telemetry"

	PartOfLabel      = "app.kubernetes.io/part-of"
	PartOfLabelValue = "k8ssandra"

	// ReplicatedByLabel is used to label secrets that should be selected for replication by a ReplicatedSecret.
	ReplicatedByLabel      = "k8ssandra.io/replicated-by"
	ReplicatedByLabelValue = "k8ssandracluster-controller"

	CleanedUpByLabel      = "k8ssandra.io/cleaned-up-by"
	CleanedUpByLabelValue = "k8ssandracluster-controller"

	K8ssandraClusterNameLabel      = "k8ssandra.io/cluster-name"
	K8ssandraClusterNamespaceLabel = "k8ssandra.io/cluster-namespace"

	DatacenterLabel = "k8ssandra.io/datacenter"
	// Forces refresh of secrets which relate to roles and authn in Cassandra.
	RefreshAnnotation = "k8ssandra.io/refresh"

	// Annotation to indicate the purpose of a given resource.
	PurposeAnnotation = "k8ssandra.io/purpose"

	// AutomatedUpdateAnnotation is an annotation that allows the Datacenters to be updated even if no changes were done to the K8ssandraCluster spec
	AutomatedUpdateAnnotation = "k8ssandra.io/autoupdate-spec"

	AllowUpdateAlways AllowUpdateType = "always"
	AllowUpdateOnce   AllowUpdateType = "once"
)

// TODO Use the accepted values from cass-operator's api instead to prevent drift, once Kubernetes dependencies are updated in k8ssandra-operator
type AllowUpdateType string

var (
	SystemKeyspaces = []string{"system_traces", "system_distributed", "system_auth"}
	DseKeyspaces    = []string{"dse_leases", "dse_perf", "dse_security"}
)
