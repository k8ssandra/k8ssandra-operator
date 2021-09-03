package secret

import (
	"testing"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestLabelIsSet(t *testing.T) {
	repSec := generateReplicatedSecret("name", "namespace", []string{"cluster-1"})
	val, exists := repSec.Labels[api.ManagedByLabel]
	require.True(t, exists)
	require.Equal(t, api.NameLabelValue, val)
}

// controllers/secret_controller_test.go tests VerifyReplicatedSecret
