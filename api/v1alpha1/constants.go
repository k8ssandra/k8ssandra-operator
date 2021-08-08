package v1alpha1

const (
	ResourceHashAnnotation = "k8ssandra.io/resource-hash"

	// StargateLabel is the distinctive label for objects created by the Stargate controller.
	StargateLabel = "k8ssandra.io/stargate"

	DefaultStargateVersion = "1.0.30"

	PartOfLabel = "app.kubernetes.io/part-of"

	PartOfLabelValue = "k8ssandra"

	K8ssandraClusterLabel = "io.k8ssandra/cluster"
)
