package secret

import (
	"github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"

	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelIsSet(t *testing.T) {
	kcKey := client.ObjectKey{Namespace: "namespace", Name: "name"}
	targets := []replicationapi.ReplicationTarget{{Namespace: "default", K8sContextName: "cluster-1"}}
	repSec := generateReplicatedSecret(kcKey, targets)

	assert.Equal(t, v1alpha1.NameLabelValue, repSec.Labels[v1alpha1.ManagedByLabel])
	assert.Equal(t, repSec.Labels[v1alpha1.K8ssandraClusterNamespaceLabel], kcKey.Namespace)
	assert.Equal(t, repSec.Labels[v1alpha1.K8ssandraClusterNameLabel], kcKey.Name)
}

func TestRandomPasswordGen(t *testing.T) {
	username, err := generateRandomString(usernameCharacters, 8)
	require.NoError(t, err)
	password, err := generateRandomString(passwordCharacters, 24)
	require.NoError(t, err)

	assert.Equal(t, 8, len(username))
	assert.Equal(t, 24, len(password))
}

func TestDefaultSuperuserSecretName(t *testing.T) {
	clusterName := "dc1"
	clusterName2 := "d-c-1"
	clusterName3 := "d_c1"

	superUsername := DefaultSuperuserSecretName(clusterName)
	superUsername2 := DefaultSuperuserSecretName(clusterName2)
	superUsername3 := DefaultSuperuserSecretName(clusterName3)

	assert.Equal(t, superUsername, superUsername2)
	assert.Equal(t, superUsername, superUsername3)

	assert.Equal(t, "dc1-superuser", superUsername)
}

func TestRequiresUpdate(t *testing.T) {
	assert := assert.New(t)
	kcKey := client.ObjectKey{Namespace: "namespace", Name: "name"}
	targets := []replicationapi.ReplicationTarget{{Namespace: "default", K8sContextName: "cluster-1"}}
	desiredRepSec := generateReplicatedSecret(kcKey, targets)

	// Labels should allow additional stuff, but must have our desired ones
	currentRepSec := generateReplicatedSecret(kcKey, targets)
	assert.False(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec.Labels["my-personal-one"] = "supersecret"
	assert.False(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec.Labels[v1alpha1.K8ssandraClusterNameLabel] = "wrong-cluster"
	assert.True(requiresUpdate(currentRepSec, desiredRepSec))

	// ReplicationTargets can include additional stuff, but not remove our targets
	currentRepSec = generateReplicatedSecret(kcKey, targets)
	assert.False(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec.Spec.ReplicationTargets = append(currentRepSec.Spec.ReplicationTargets, replicationapi.ReplicationTarget{
		K8sContextName: "some-distant-backup-galaxy",
	})
	assert.False(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec.Spec.ReplicationTargets = []replicationapi.ReplicationTarget{
		{
			K8sContextName: "some-distant-backup-galaxy",
		},
	}
	assert.True(requiresUpdate(currentRepSec, desiredRepSec))

	// Selector must always match what we have, nothing additional even allowed
	currentRepSec = generateReplicatedSecret(kcKey, targets)
	assert.False(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec.Spec.Selector.MatchLabels["new-matcher"] = "my-only-work"
	assert.True(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec = generateReplicatedSecret(kcKey, targets)
	assert.False(requiresUpdate(currentRepSec, desiredRepSec))

	currentRepSec.Spec.Selector.MatchLabels = make(map[string]string)
	assert.True(requiresUpdate(currentRepSec, desiredRepSec))
}

// controllers/secret_controller_test.go tests VerifyReplicatedSecret
