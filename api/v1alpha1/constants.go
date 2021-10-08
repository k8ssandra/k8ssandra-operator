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

	CreatedByLabel                                = "app.kubernetes.io/created-by"
	CreatedByLabelValueK8ssandraClusterController = "k8ssandracluster-controller"
	CreatedByLabelValueStargateController         = "stargate-controller"

	PartOfLabel      = "app.kubernetes.io/part-of"
	PartOfLabelValue = "k8ssandra"

	K8ssandraClusterLabel = "k8ssandra.io/cluster"

	// StargateLabel is the distinctive label for all objects created by the Stargate controller. The label value is
	// the Stargate resource name.
	StargateLabel = "k8ssandra.io/stargate"

	// StargateDeploymentLabel is a distinctive label for pods targeted by a deployment created by the Stargate
	// controller. The label value is the Deployment name.
	StargateDeploymentLabel = "k8ssandra.io/stargate-deployment"

	DefaultStargateVersion = "1.0.36"
)
