package secret

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"

	"github.com/go-logr/logr"
	"github.com/k8ssandra/k8ssandra-operator/pkg/annotations"
	"github.com/k8ssandra/k8ssandra-operator/pkg/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
)

const (
	passwordCharacters = "-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	usernameCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// OrphanResourceAnnotation when set to true prevents the deletion of secret from target clusters even if matching ReplicatedSecret is removed
	OrphanResourceAnnotation = "replicatedresource.k8ssandra.io/orphan"
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

// DefaultSuperuserSecretName returns a default name for the superuser secret. It is of the general form
// <cluster_name>-superuser. Note that we expect clusterName to be a valid DNS subdomain name as defined in RFC 1123,
// otherwise the secret name won't be valid.
func DefaultSuperuserSecretName(clusterName string) string {
	return fmt.Sprintf("%s-superuser", clusterName)
}

// ReconcileSecret creates a new secret with proper "managed-by" annotations, or ensure the existing secret has such
// annotations.
func ReconcileSecret(ctx context.Context, c client.Client, secretName string, kcKey client.ObjectKey) error {
	if secretName == "" {
		return fmt.Errorf("secretName is required")
	}
	currentSec := &corev1.Secret{}
	err := c.Get(ctx, types.NamespacedName{Name: secretName, Namespace: kcKey.Namespace}, currentSec)
	if err != nil {
		if errors.IsNotFound(err) {
			password, err := generateRandomString(passwordCharacters, 20)
			if err != nil {
				return err
			}

			sec := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: kcKey.Namespace,
					Labels:    labels.ReplicatedByLabels(kcKey),
				},
				Type: "Opaque",
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

	// It exists or was created: ensure it has proper annotations
	if !labels.IsReplicatedBy(currentSec, kcKey) {
		labels.SetReplicatedBy(currentSec, kcKey)
		annotations.AddAnnotation(currentSec, OrphanResourceAnnotation, "true")
		return c.Update(ctx, currentSec)
	}
	return nil
}

// ReconcileReplicatedSecret ensures that the correct replicatedSecret for all managed secrets is created
func ReconcileReplicatedSecret(ctx context.Context, c client.Client, scheme *runtime.Scheme, kc *api.K8ssandraCluster, logger logr.Logger) error {
	replicationTargets := make([]replicationapi.ReplicationTarget, 0, len(kc.Spec.Cassandra.Datacenters))
	for _, dcTemplate := range kc.Spec.Cassandra.Datacenters {
		if dcTemplate.K8sContext != "" || dcTemplate.Meta.Namespace != "" {
			replicationTargets = append(replicationTargets, replicationapi.ReplicationTarget{
				K8sContextName: dcTemplate.K8sContext,
				Namespace:      dcTemplate.Meta.Namespace,
			})
		}
	}

	targetRepSec := generateReplicatedSecret(client.ObjectKey{Namespace: kc.Namespace, Name: kc.Name}, replicationTargets)
	key := client.ObjectKey{Namespace: targetRepSec.Namespace, Name: targetRepSec.Name}
	repSec := &replicationapi.ReplicatedSecret{}

	err := controllerutil.SetControllerReference(kc, targetRepSec, scheme)
	if err != nil {
		logger.Error(err, "Failed to set owner reference on ReplicatedSecret", "ReplicatedSecret", key)
		return err
	}

	err = c.Get(ctx, key, repSec)
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
	if requiresUpdate(repSec, targetRepSec) {
		currentResourceVersion := repSec.ResourceVersion
		// Need to copy the finalizers here; otherwise, they get overwritten and lost. This
		// will be refactored in https://github.com/k8ssandra/k8ssandra-operator/issues/206
		finalizers := repSec.Finalizers
		targetRepSec.DeepCopyInto(repSec)
		repSec.ResourceVersion = currentResourceVersion
		repSec.Finalizers = finalizers
		return c.Update(ctx, repSec)
	}

	return nil
}

func HasReplicatedSecrets(ctx context.Context, c client.Client, kcKey client.ObjectKey, targetContext string) bool {
	if targetContext == "" {
		return true
	}

	repSec := &replicationapi.ReplicatedSecret{}
	err := c.Get(ctx, types.NamespacedName{Name: kcKey.Name, Namespace: kcKey.Namespace}, repSec)
	if err != nil {
		return false
	}

	for _, cond := range repSec.Status.Conditions {
		if cond.Cluster == targetContext && cond.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

func generateReplicatedSecret(kcKey client.ObjectKey, replicationTargets []replicationapi.ReplicationTarget) *replicationapi.ReplicatedSecret {
	return &replicationapi.ReplicatedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kcKey.Name,
			Namespace: kcKey.Namespace,
			Labels:    labels.WatchedByK8ssandraClusterLabels(kcKey),
		},
		Spec: replicationapi.ReplicatedSecretSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels.ReplicatedByLabels(kcKey),
			},
			ReplicationTargets: replicationTargets,
		},
	}
}

func requiresUpdate(current, desired *replicationapi.ReplicatedSecret) bool {
	// Ensure our labels are there (allow additionals) and our selector is there (nothing else) and replicationTargets has at least our targets
	// Selector must not change
	if !reflect.DeepEqual(current.Spec.Selector, desired.Spec.Selector) {
		return true
	}

	// Labels must have our labels, additionals allowed
	for k, v := range desired.Labels {
		if current.Labels[k] != v {
			return true
		}
	}

	// ReplicationTargets must be different to require an update
	return !reflect.DeepEqual(current.Spec.ReplicationTargets, desired.Spec.ReplicationTargets)
}
