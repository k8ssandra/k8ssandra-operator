package secret

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"math/big"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
//func ReconcileReplicatedSecret(ctx context.Context, c client.Client, clusterName, namespace string, targetContexts []string) (*api.ReplicatedSecret, error) {
func ReconcileReplicatedSecret(ctx context.Context, c client.Client, scheme *runtime.Scheme, kc *api.K8ssandraCluster, logger logr.Logger) error {
	replicationTargets := make([]string, 0, len(kc.Spec.Cassandra.Datacenters))
	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		if dcTemplate.K8sContext != "" {
			replicationTargets = append(replicationTargets, dcTemplate.K8sContext)
		}
	}

	targetRepSec := generateReplicatedSecret(kc.Spec.Cassandra.Cluster, kc.Namespace, replicationTargets)
	key := client.ObjectKey{Namespace: targetRepSec.Namespace, Name: targetRepSec.Name}
	repSec := &replicationapi.ReplicatedSecret{}

	err := controllerutil.SetControllerReference(kc, targetRepSec, scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on ReplicatedSecret", "ReplicatedSect", key)
		return err
	}

	err = c.Get(ctx, types.NamespacedName{Name: kc.Spec.Cassandra.Cluster, Namespace: kc.Namespace}, repSec)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating ReplicatedSecret", "ReplicatedSecret", key)
			if err = c.Create(ctx, targetRepSec); err == nil {
				return nil
			}
		}
		return err
	}

	// It exists, override whatever was in it
	currentResourceVersion := repSec.ResourceVersion
	finalizers := repSec.Finalizers
	targetRepSec.DeepCopyInto(repSec)
	repSec.ResourceVersion = currentResourceVersion
	repSec.Finalizers = finalizers
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
