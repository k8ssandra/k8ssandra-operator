package secret

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/k8ssandra/k8ssandra-operator/api/v1alpha1"
)

func CreateSuperuserSecret(clusterName, namespace string) *corev1.Secret {
	// TODO Add the ReplicatedSecret to Kustomize, with only matching label used

	/*
		Generate superUserName
		Generate superUserPassword
		Insert
	*/

	sec := &corev1.Secret{
		ObjectMeta: getManagedObjectMeta(clusterName, namespace),
	}

	return sec
}

// VerifyReplicatedSecret ensures that the correct replicatedSecret for all managed secrets is created
func VerifyReplicatedSecret(c client.Client, leaderID, namespace string, targetContexts []string) error {
	// We use leaderID to ensure that this leader instance of k8ssandra-operator is using the correct ReplicatedSecret (it can be shared between different k8ssandra-operators)

	targetRepSec := generateReplicatedSecret(leaderID, namespace, targetContexts)
	repSec := &api.ReplicatedSecret{}
	err := c.Get(context.Background(), types.NamespacedName{Name: leaderID, Namespace: namespace}, repSec)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.Create(context.Background(), targetRepSec)
		}
	}

	// It exists, override whatever was in it
	targetRepSec.DeepCopyInto(repSec)
	return c.Update(context.Background(), repSec)
}

func generateReplicatedSecret(name, namespace string, targetContexts []string) *api.ReplicatedSecret {

	replicationTargets := make([]api.ReplicationTarget, 0, len(targetContexts))
	for _, ctx := range targetContexts {
		replicationTargets = append(replicationTargets, api.ReplicationTarget{
			K8sContextName: ctx,
		})
	}

	return &api.ReplicatedSecret{
		ObjectMeta: getManagedObjectMeta(name, namespace),
		Spec: api.ReplicatedSecretSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{api.ManagedByLabel: api.NameLabelValue},
			},
			ReplicationTargets: replicationTargets,
		},
	}
}

func getManagedObjectMeta(name, namespace string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels: map[string]string{
			api.ManagedByLabel: api.NameLabelValue,
		},
	}
}
