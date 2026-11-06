package v1alpha1

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseTargetTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{name: "RFC3339 UTC", input: "2024-07-15T12:30:00Z", want: time.Date(2024, 7, 15, 12, 30, 0, 0, time.UTC)},
		{name: "RFC3339 fractional", input: "2026-11-03T10:35:11.123Z", want: time.Date(2026, 11, 3, 10, 35, 11, 123000000, time.UTC)},
		{name: "RFC3339 offset converted to UTC", input: "2024-07-15T14:30:00+02:00", want: time.Date(2024, 7, 15, 12, 30, 0, 0, time.UTC)},
		{name: "ISO-8601 without timezone is UTC", input: "2024-07-15T12:30:00", want: time.Date(2024, 7, 15, 12, 30, 0, 0, time.UTC)},
		{name: "ISO-8601 without timezone fractional", input: "2024-07-15T12:30:00.5", want: time.Date(2024, 7, 15, 12, 30, 0, 500000000, time.UTC)},
		{name: "epoch float", input: "1721046600.5", want: time.Date(2024, 7, 15, 12, 30, 0, 500000000, time.UTC)},
		{name: "epoch integer", input: "1721046600", want: time.Date(2024, 7, 15, 12, 30, 0, 0, time.UTC)},
		{name: "words", input: "yesterday", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "NaN", input: "NaN", wantErr: true},
		{name: "Inf", input: "+Inf", wantErr: true},
		{name: "date only", input: "2024-07-15", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTargetTimestamp(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "spec.targetTimestamp")
				return
			}
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "want %v, got %v", tt.want, got)
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

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
	ctx := context.Background()

	job := func(backup, ts string) *MedusaRestoreJob {
		return &MedusaRestoreJob{
			ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
			Spec:       MedusaRestoreJobSpec{Backup: backup, TargetTimestamp: ts, CassandraDatacenter: "dc1"},
		}
	}

	t.Run("create valid", func(t *testing.T) {
		_, err := v.ValidateCreate(ctx, job("b1", ""))
		require.NoError(t, err)
		_, err = v.ValidateCreate(ctx, job("", "2024-07-15T12:30:00Z"))
		require.NoError(t, err)
	})

	t.Run("create invalid", func(t *testing.T) {
		_, err := v.ValidateCreate(ctx, job("b1", "2024-07-15T12:30:00Z"))
		require.Error(t, err)
		_, err = v.ValidateCreate(ctx, job("", ""))
		require.Error(t, err)
		_, err = v.ValidateCreate(ctx, job("", "yesterday"))
		require.Error(t, err)
	})

	t.Run("update valid to invalid", func(t *testing.T) {
		_, err := v.ValidateUpdate(ctx, job("b1", ""), job("b1", "2024-07-15T12:30:00Z"))
		require.Error(t, err)
	})

	t.Run("update valid to valid", func(t *testing.T) {
		_, err := v.ValidateUpdate(ctx, job("b1", ""), job("b2", ""))
		require.NoError(t, err)
	})

	t.Run("update with unchanged spec is allowed", func(t *testing.T) {
		old := job("", "")
		updated := old.DeepCopy()
		updated.Finalizers = nil
		_, err := v.ValidateUpdate(ctx, old, updated)
		require.NoError(t, err)
	})

	t.Run("delete", func(t *testing.T) {
		_, err := v.ValidateDelete(ctx, job("", ""))
		require.NoError(t, err)
	})
}
