package medusa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPurgeTimeout(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        time.Duration
		wantErr     bool
	}{
		{name: "not set", want: DefaultPurgeTimeout},
		{name: "set", annotations: map[string]string{MedusaPurgeTimeoutAnnotation: "36h"}, want: 36 * time.Hour},
		{name: "invalid falls back to default", annotations: map[string]string{MedusaPurgeTimeoutAnnotation: "tomorrow"}, want: DefaultPurgeTimeout, wantErr: true},
		{name: "zero falls back to default", annotations: map[string]string{MedusaPurgeTimeoutAnnotation: "0s"}, want: DefaultPurgeTimeout, wantErr: true},
		{name: "negative falls back to default", annotations: map[string]string{MedusaPurgeTimeoutAnnotation: "-1h"}, want: DefaultPurgeTimeout, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{Annotations: tt.annotations}
			got, err := PurgeTimeout(obj)
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCopyPurgeTaskAnnotations(t *testing.T) {
	src := &metav1.ObjectMeta{Annotations: map[string]string{
		MedusaPurgeTimeoutAnnotation: "36h",
		"unrelated":                  "value",
	}}

	dst := &metav1.ObjectMeta{}
	CopyPurgeTaskAnnotations(src, dst)
	assert.Equal(t, map[string]string{MedusaPurgeTimeoutAnnotation: "36h"}, dst.Annotations)

	dst = &metav1.ObjectMeta{Annotations: map[string]string{"existing": "value"}}
	CopyPurgeTaskAnnotations(src, dst)
	assert.Equal(t, map[string]string{"existing": "value", MedusaPurgeTimeoutAnnotation: "36h"}, dst.Annotations)

	dst = &metav1.ObjectMeta{}
	CopyPurgeTaskAnnotations(&metav1.ObjectMeta{}, dst)
	assert.Nil(t, dst.Annotations)
}

func TestWithDefaultTimeout(t *testing.T) {
	before := time.Now()
	ctx, cancel := withDefaultTimeout(t.Context(), DefaultCallTimeout)
	defer cancel()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	assert.WithinDuration(t, before.Add(DefaultCallTimeout), deadline, time.Second)

	// A deadline set by the caller is kept, even when it is longer than the default.
	callerDeadline := time.Now().Add(DefaultPurgeTimeout)
	parent, parentCancel := context.WithDeadline(t.Context(), callerDeadline)
	defer parentCancel()
	ctx, cancel = withDefaultTimeout(parent, DefaultCallTimeout)
	defer cancel()
	deadline, ok = ctx.Deadline()
	require.True(t, ok)
	assert.Equal(t, callerDeadline, deadline)
}
