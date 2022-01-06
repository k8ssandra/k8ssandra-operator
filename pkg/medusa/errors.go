package medusa

import "errors"

var (
	// This error indicates that a pod (or pods) do not include the medusa backup sidecar
	// container.
	BackupSidecarNotFound = errors.New("the backup sidecar was not found")
)
