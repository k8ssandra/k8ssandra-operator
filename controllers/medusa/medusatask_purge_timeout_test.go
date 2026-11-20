package medusa

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// blockingPurgeClient blocks PurgeBackups until its context is done, like a call to a Medusa container that went
// away, and records the deadline it was given.
type blockingPurgeClient struct {
	medusa.Client
	deadline    time.Time
	hasDeadline bool
}

func (c *blockingPurgeClient) PurgeBackups(ctx context.Context) (*medusa.PurgeBackupsResponse, error) {
	c.deadline, c.hasDeadline = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (c *blockingPurgeClient) Close() error { return nil }

type blockingPurgeClientFactory struct {
	client *blockingPurgeClient
}

func (f *blockingPurgeClientFactory) NewClient(string) (medusa.Client, error) { return f.client, nil }

func (f *blockingPurgeClientFactory) NewClientWithTLS(string, *corev1.Secret) (medusa.Client, error) {
	return f.client, nil
}

func TestDoPurgeIsBounded(t *testing.T) {
	cassdc := &cassdcapi.CassandraDatacenter{ObjectMeta: metav1.ObjectMeta{Name: "dc1", Namespace: "test-ns"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-0", Namespace: "test-ns"}}
	c := fake.NewClientBuilder().Build()

	t.Run("default timeout", func(t *testing.T) {
		// Cancel the parent so the blocked call returns; the deadline is recorded before that.
		ctx, cancel := context.WithCancel(t.Context())
		factory := &blockingPurgeClientFactory{client: &blockingPurgeClient{}}
		task := &medusav1alpha1.MedusaTask{ObjectMeta: metav1.ObjectMeta{Name: "purge", Namespace: "test-ns"}}

		before := time.Now()
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		_, err := doPurge(ctx, task, pod, cassdc, c, factory, logr.Discard())
		require.ErrorIs(t, err, context.Canceled)
		require.True(t, factory.client.hasDeadline)
		assert.WithinDuration(t, before.Add(medusa.DefaultPurgeTimeout), factory.client.deadline, time.Minute)
	})

	t.Run("annotation timeout", func(t *testing.T) {
		factory := &blockingPurgeClientFactory{client: &blockingPurgeClient{}}
		task := &medusav1alpha1.MedusaTask{ObjectMeta: metav1.ObjectMeta{
			Name:        "purge",
			Namespace:   "test-ns",
			Annotations: map[string]string{medusa.MedusaPurgeTimeoutAnnotation: "200ms"},
		}}

		_, err := doPurge(t.Context(), task, pod, cassdc, c, factory, logr.Discard())
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
