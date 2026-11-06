package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateRestoreJobSpec(t *testing.T) {
	tests := []struct {
		name      string
		spec      MedusaRestoreJobSpec
		errSubstr string
	}{
		{name: "backup only", spec: MedusaRestoreJobSpec{Backup: "b1", CassandraDatacenter: "dc1"}},
		{name: "ISO-8601 UTC timestamp only", spec: MedusaRestoreJobSpec{TargetTimestamp: "2024-07-15T12:30:00Z", CassandraDatacenter: "dc1"}},
		{name: "epoch float timestamp only", spec: MedusaRestoreJobSpec{TargetTimestamp: "1721046600.5", CassandraDatacenter: "dc1"}},
		{name: "both set", spec: MedusaRestoreJobSpec{Backup: "b1", TargetTimestamp: "2024-07-15T12:30:00Z", CassandraDatacenter: "dc1"}, errSubstr: "mutually exclusive"},
		{name: "neither set", spec: MedusaRestoreJobSpec{CassandraDatacenter: "dc1"}, errSubstr: "must be set"},
		{name: "invalid timestamp", spec: MedusaRestoreJobSpec{TargetTimestamp: "yesterday", CassandraDatacenter: "dc1"}, errSubstr: "spec.targetTimestamp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRestoreJobSpec(&tt.spec)
			if tt.errSubstr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

func TestMedusaRestoreJobValidator(t *testing.T) {
	v := &MedusaRestoreJobValidator{}

	job := func(backup, ts string) *MedusaRestoreJob {
		return &MedusaRestoreJob{
			ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
			Spec:       MedusaRestoreJobSpec{Backup: backup, TargetTimestamp: ts, CassandraDatacenter: "dc1"},
		}
	}

	t.Run("create valid", func(t *testing.T) {
		_, err := v.ValidateCreate(t.Context(), job("b1", ""))
		require.NoError(t, err)
		_, err = v.ValidateCreate(t.Context(), job("", "2024-07-15T12:30:00Z"))
		require.NoError(t, err)
	})

	t.Run("create invalid", func(t *testing.T) {
		_, err := v.ValidateCreate(t.Context(), job("b1", "2024-07-15T12:30:00Z"))
		require.Error(t, err)
		_, err = v.ValidateCreate(t.Context(), job("", ""))
		require.Error(t, err)
		_, err = v.ValidateCreate(t.Context(), job("", "yesterday"))
		require.Error(t, err)
	})

	t.Run("update valid to invalid", func(t *testing.T) {
		_, err := v.ValidateUpdate(t.Context(), job("b1", ""), job("b1", "2024-07-15T12:30:00Z"))
		require.Error(t, err)
	})

	t.Run("update valid to valid", func(t *testing.T) {
		_, err := v.ValidateUpdate(t.Context(), job("b1", ""), job("b2", ""))
		require.NoError(t, err)
	})

	t.Run("update with unchanged spec is allowed", func(t *testing.T) {
		old := job("", "")
		updated := old.DeepCopy()
		updated.Finalizers = nil
		_, err := v.ValidateUpdate(t.Context(), old, updated)
		require.NoError(t, err)
	})

	t.Run("delete", func(t *testing.T) {
		_, err := v.ValidateDelete(t.Context(), job("", ""))
		require.NoError(t, err)
	})
}
