package leancache

import (
	"context"
	"errors"
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	controlapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	kcapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	repapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// cacheStub stands in for the cache-backed client: Gets return err (or hit the live client
// when err is nil), writes go to the live client.
type cacheStub struct {
	client.Client
	err error
}

func (c *cacheStub) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.err != nil {
		return c.err
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

type countingReader struct {
	client.Reader
	gets int
}

func (r *countingReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	r.gets++
	return r.Reader.Get(ctx, key, obj, opts...)
}

func newScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	return s
}

func TestRead(t *testing.T) {
	key := client.ObjectKey{Namespace: "ns", Name: "credentials"}
	notFound := apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, key.Name)
	cacheFailure := errors.New("cache failed")
	for _, tc := range []struct {
		name          string
		cacheErr      error
		alreadyLabels bool
		wantErr       error
		wantLiveGets  int
		wantLabel     bool
	}{
		{name: "cache hit", wantLiveGets: 0},
		{name: "cache miss reads live and labels", cacheErr: notFound, wantLiveGets: 1, wantLabel: true},
		{name: "cache miss on already labelled object", cacheErr: notFound, alreadyLabels: true, wantLiveGets: 1, wantLabel: true},
		{name: "cache failure is not retried live", cacheErr: cacheFailure, wantErr: cacheFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name,
					Annotations: map[string]string{corev1.LastAppliedConfigAnnotation: "original"}},
				Data: map[string][]byte{"password": []byte("keep")},
			}
			if tc.alreadyLabels {
				MarkWatched(secret)
			}
			live := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(secret).Build()
			reader := &countingReader{Reader: live}
			c := &cachedClient{Client: &cacheStub{Client: live, err: tc.cacheErr}, apiReader: reader}

			got := &corev1.Secret{}
			err := Read(t.Context(), c, key, got)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Zero(t, reader.gets)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantLiveGets, reader.gets)
			require.Equal(t, "keep", string(got.Data["password"]))

			stored := &corev1.Secret{}
			require.NoError(t, live.Get(t.Context(), key, stored))
			if tc.wantLabel {
				require.Equal(t, "true", stored.Labels[WatchedLabel])
			}
			require.Equal(t, "original", stored.Annotations[corev1.LastAppliedConfigAnnotation], "labelling must not touch the apply baseline")
		})
	}
}

func TestReadWithoutLiveReader(t *testing.T) {
	// Clients that are not created by NewClient (fake or uncached ones) are authoritative.
	c := fake.NewClientBuilder().WithScheme(newScheme(t)).Build()
	err := Read(t.Context(), c, client.ObjectKey{Namespace: "ns", Name: "absent"}, &corev1.Secret{})
	require.True(t, apierrors.IsNotFound(err))
}

func TestReadingClient(t *testing.T) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "tls"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pod"}}
	live := fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(secret, pod).Build()
	reader := &countingReader{Reader: live}
	notFound := apierrors.NewNotFound(schema.GroupResource{}, "")
	c := ReadingClient(&cachedClient{Client: &cacheStub{Client: live, err: notFound}, apiReader: reader})

	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(secret), &corev1.Secret{}))
	require.Equal(t, 1, reader.gets, "Secrets go through Read")
	require.True(t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKeyFromObject(pod), &corev1.Pod{})))
	require.Equal(t, 1, reader.gets, "other types use the cache only")

	plain := fake.NewClientBuilder().Build()
	require.Same(t, plain, ReadingClient(plain))
}

func TestOptionsSelectors(t *testing.T) {
	opts := Options()
	var managed cache.ByObject
	for obj, cfg := range opts.ByObject {
		if _, ok := obj.(*corev1.Pod); ok {
			managed = cfg
		}
	}
	require.NotNil(t, managed.Label)
	require.True(t, managed.Label.Matches(labels.Set{kcapi.NameLabel: "cassandra"}))
	require.True(t, managed.Label.Matches(labels.Set{kcapi.NameLabel: kcapi.NameLabelValue}))
	require.False(t, managed.Label.Matches(labels.Set{kcapi.NameLabel: "something-else"}))
	require.False(t, managed.Label.Matches(labels.Set{}))

	for obj, cfg := range opts.ByObject {
		switch obj.(type) {
		case *corev1.Secret, *corev1.ConfigMap:
			require.True(t, cfg.Label.Matches(labels.Set{WatchedLabel: "true"}))
			require.False(t, cfg.Label.Matches(labels.Set{kcapi.NameLabel: kcapi.NameLabelValue}))
		}
	}
}

func TestStripHeavyMetadata(t *testing.T) {
	for _, obj := range []client.Object{
		&kcapi.K8ssandraCluster{}, &controlapi.K8ssandraTask{}, &repapi.ReplicatedSecret{}, &medusaapi.MedusaTask{}, &medusaapi.MedusaConfiguration{},
		&corev1.Secret{}, &corev1.ConfigMap{}, &cassdcapi.CassandraDatacenter{}, &corev1.Service{},
		&metav1.PartialObjectMetadata{},
	} {
		obj.SetAnnotations(map[string]string{corev1.LastAppliedConfigAnnotation: "original", "other": "keep"})
		obj.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl"}})
		_, err := StripHeavyMetadata()(obj)
		require.NoError(t, err)
		require.Empty(t, obj.GetManagedFields())
		require.Equal(t, "keep", obj.GetAnnotations()["other"])
		switch obj.(type) {
		case *cassdcapi.CassandraDatacenter, *corev1.Service, *metav1.PartialObjectMetadata:
			require.NotContains(t, obj.GetAnnotations(), corev1.LastAppliedConfigAnnotation, "%T", obj)
		default:
			require.Equal(t, "original", obj.GetAnnotations()[corev1.LastAppliedConfigAnnotation], "%T", obj)
		}
	}
}
