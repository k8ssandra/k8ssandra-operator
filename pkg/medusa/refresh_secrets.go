package medusa

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/reconciliation"
	"github.com/k8ssandra/k8ssandra-operator/pkg/result"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	maxRetries = 5
)

func RefreshSecrets(dc *cassdcapi.CassandraDatacenter, ctx context.Context, client client.Client, logger logr.Logger, requeueDelay time.Duration) result.ReconcileResult {
	logger.Info(fmt.Sprintf("Restore complete for DC %#v, Refreshing secrets", dc.ObjectMeta))
	userSecrets := []string{}
	for _, user := range dc.Spec.Users {
		userSecrets = append(userSecrets, user.SecretName)
	}
	if dc.Spec.SuperuserSecretName == "" {
		userSecrets = append(userSecrets, cassdcapi.CleanupForKubernetes(dc.Spec.ClusterName)+"-superuser") //default SU secret
	} else {
		userSecrets = append(userSecrets, dc.Spec.SuperuserSecretName)
	}
	logger.Info(fmt.Sprintf("refreshing user secrets for %v", userSecrets))
	//  Both Reaper and medusa secrets go into the userSecrets, so they don't need special handling.
	timestamp := time.Now().String()
	for _, i := range userSecrets {
		secret := &corev1.Secret{}
		// We need to do our own retries here instead of delegating it back up to the reconciler, because of
		// the nature (time based) of the annotation we're adding. Otherwise we never complete because the
		// object on the server never matches the desired object with the new time.
		retries := 0
	InnerRetryLoop:
		for retries <= maxRetries {
			err := client.Get(ctx, types.NamespacedName{Name: i, Namespace: dc.Namespace}, secret)
			if err != nil {
				logger.Error(err, fmt.Sprintf("Failed to get secret %s", i))
				return result.Error(err)
			}
			if secret.ObjectMeta.Annotations == nil {
				secret.ObjectMeta.Annotations = make(map[string]string)
			}
			secret.ObjectMeta.Annotations[k8ssandraapi.RefreshAnnotation] = timestamp
			recRes := reconciliation.ReconcileObject(ctx, client, requeueDelay, *secret)
			switch {
			case recRes.IsError():
				return recRes
			case recRes.IsRequeue():
				retries++
				continue
			case recRes.IsDone():
				break InnerRetryLoop
			}
		}
		if retries == maxRetries {
			return result.Error(fmt.Errorf("failed to refresh secret %s after %d retries", i, maxRetries))
		}
	}
	return result.Done()
}
