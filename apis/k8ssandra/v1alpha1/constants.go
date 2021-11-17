package v1alpha1

const (
	ResourceHashAnnotation = "k8ssandra.io/resource-hash"

	NameLabel      = "app.kubernetes.io/name"
	NameLabelValue = "k8ssandra-operator"

	InstanceLabel  = "app.kubernetes.io/instance"
	VersionLabel   = "app.kubernetes.io/version"
	ManagedByLabel = "app.kubernetes.io/managed-by"

	ComponentLabel               = "app.kubernetes.io/component"
	ComponentLabelValueCassandra = "cassandra"
	ComponentLabelValueStargate  = "stargate"
	ComponentLabelValueReaper    = "reaper"

	CreatedByLabel                                = "app.kubernetes.io/created-by"
	CreatedByLabelValueK8ssandraClusterController = "k8ssandracluster-controller"
	CreatedByLabelValueStargateController         = "stargate-controller"
	CreatedByLabelValueReaperController           = "reaper-controller"

	PartOfLabel      = "app.kubernetes.io/part-of"
	PartOfLabelValue = "k8ssandra"

	K8ssandraClusterNameLabel      = "k8ssandra.io/cluster-name"
	K8ssandraClusterNamespaceLabel = "k8ssandra.io/cluster-namespace"
)
