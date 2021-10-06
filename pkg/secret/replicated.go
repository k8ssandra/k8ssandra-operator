package secret

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
)

const (
	passwordCharacters = "!#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	usernameCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
)

func generateRandomString(charset string, length int) ([]byte, error) {
	maxInt := big.NewInt(int64(len(charset)))
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, maxInt)
		if err != nil {
			return nil, err
		}
		result[i] = charset[n.Int64()]
	}

	return result, nil
}

// DefaultSuperuserSecretName follows the convention from k8ssandra Helm charts
func DefaultSuperuserSecretName(clusterName string) string {
	cleanedClusterName := strings.ReplaceAll(strings.ReplaceAll(clusterName, "_", ""), "-", "")
	secretName := fmt.Sprintf("%s-superuser", cleanedClusterName)

	return secretName
}

// ReconcileSuperuserSecret creates the superUserSecret with proper annotations
func ReconcileSuperuserSecret(ctx context.Context, c client.Client, secretName, clusterName, namespace string) error {
	if secretName == "" {
		return fmt.Errorf("secretName is required")
	}
	currentSec := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, currentSec)
	if err != nil {
		if errors.IsNotFound(err) {
			password, err := generateRandomString(passwordCharacters, 20)
			if err != nil {
				return err
			}

			sec := &corev1.Secret{
				ObjectMeta: getManagedObjectMeta(secretName, namespace, clusterName),
				Type:       "Opaque",
				// Immutable feature is only available from 1.21 and up (beta in 1.19 and up)
				// Immutable:  true,
				Data: map[string][]byte{
					"username": []byte(secretName),
					"password": password,
				},
			}

			return c.Create(ctx, sec)
		} else {
			return err
		}
	}

	// It exists or was created, we won't modify it anymore
	return nil
}

// ReconcileReplicatedSecret ensures that the correct replicatedSecret for all managed secrets is created
func ReconcileReplicatedSecret(ctx context.Context, c client.Client, clusterName, namespace string, targetContexts []string) error {
	// We use leaderID to ensure that this leader instance of k8ssandra-operator is using the correct ReplicatedSecret (it can be shared between different k8ssandra-operators)
	targetRepSec := generateReplicatedSecret(clusterName, namespace, targetContexts)
	repSec := &replicationapi.ReplicatedSecret{}
	err := c.Get(ctx, types.NamespacedName{Name: clusterName, Namespace: namespace}, repSec)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(ctx, targetRepSec)
		}
	}

	// It exists, override whatever was in it
	currentResourceVersion := repSec.ResourceVersion
	targetRepSec.DeepCopyInto(repSec)
	repSec.ResourceVersion = currentResourceVersion
	return c.Update(ctx, repSec)
}

func generateReplicatedSecret(clusterName, namespace string, targetContexts []string) *replicationapi.ReplicatedSecret {

	replicationTargets := make([]replicationapi.ReplicationTarget, 0, len(targetContexts))
	for _, ctx := range targetContexts {
		replicationTargets = append(replicationTargets, replicationapi.ReplicationTarget{
			K8sContextName: ctx,
		})
	}

	return &replicationapi.ReplicatedSecret{
		ObjectMeta: getManagedObjectMeta(clusterName, namespace, clusterName),
		Spec: replicationapi.ReplicatedSecretSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					api.ManagedByLabel:        api.NameLabelValue,
					api.K8ssandraClusterLabel: clusterName,
				},
			},
			ReplicationTargets: replicationTargets,
		},
	}
}

func getManagedObjectMeta(name, namespace, clusterName string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			api.ManagedByLabel:        api.NameLabelValue,
			api.K8ssandraClusterLabel: clusterName,
		},
	}
}
