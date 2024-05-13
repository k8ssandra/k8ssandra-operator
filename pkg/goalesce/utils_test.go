package goalesceutils

import (
	"testing"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestMergeCRs(t *testing.T) {
	type Foo struct {
		Enabled *bool
		Text    string
		Number  *int
	}
	tests := []struct {
		name    string
		cluster interface{}
		dc      interface{}
		want    interface{}
	}{
		// generic
		{
			name:    "nil",
			cluster: nil,
			dc:      nil,
			want:    nil,
		},
		{
			name:    "nil vs non-nil",
			cluster: nil,
			dc:      Foo{Enabled: ptr.To(true), Text: "foo", Number: ptr.To[int](123)},
			want:    Foo{Enabled: ptr.To(true), Text: "foo", Number: ptr.To[int](123)},
		},
		{
			name:    "non-nil vs nil",
			cluster: Foo{Enabled: ptr.To(true), Text: "foo", Number: ptr.To[int](123)},
			dc:      nil,
			want:    Foo{Enabled: ptr.To(true), Text: "foo", Number: ptr.To[int](123)},
		},
		{
			name:    "non-nil vs non-nil",
			cluster: Foo{Enabled: ptr.To(true), Text: "foo", Number: ptr.To[int](123)},
			dc:      Foo{Enabled: ptr.To(false), Text: "bar", Number: ptr.To[int](456)},
			want:    Foo{Enabled: ptr.To(false), Text: "bar", Number: ptr.To[int](456)},
		},
		// special cases
		{
			name:    "volume source",
			cluster: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			dc:      corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{}},
			want:    corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{}},
		},
		{
			name:    "env var source",
			cluster: corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{}},
			dc:      corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{}},
			want:    corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{}},
		},
		{
			name:    "container",
			cluster: []corev1.Container{{Name: "foo", Image: "foo", ImagePullPolicy: corev1.PullAlways}},
			dc:      []corev1.Container{{Name: "foo", Image: "bar"}},
			want:    []corev1.Container{{Name: "foo", Image: "bar", ImagePullPolicy: corev1.PullAlways}},
		},
		{
			name:    "container port",
			cluster: []corev1.ContainerPort{{ContainerPort: 123, Name: "foo", HostPort: 456}},
			dc:      []corev1.ContainerPort{{ContainerPort: 123, Name: "bar"}},
			want:    []corev1.ContainerPort{{ContainerPort: 123, Name: "bar", HostPort: 456}},
		},
		{
			name:    "env vars",
			cluster: []corev1.EnvVar{{Name: "foo", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}}},
			dc:      []corev1.EnvVar{{Name: "foo", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{}}}},
			want:    []corev1.EnvVar{{Name: "foo", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{}}}},
		},
		{
			name:    "env vars 2",
			cluster: []corev1.EnvVar{{Name: "foo", Value: "bar"}},
			dc:      []corev1.EnvVar{{Name: "foo", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}}},
			want:    []corev1.EnvVar{{Name: "foo", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}}},
		},
		{
			name:    "volume",
			cluster: []corev1.Volume{{Name: "foo", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
			dc:      []corev1.Volume{{Name: "foo", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{}}}},
			want:    []corev1.Volume{{Name: "foo", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{}}}},
		},
		{
			name:    "volume mount",
			cluster: []corev1.VolumeMount{{MountPath: "foo", Name: "foo", SubPath: "foo"}},
			dc:      []corev1.VolumeMount{{MountPath: "foo", Name: "bar"}},
			want:    []corev1.VolumeMount{{MountPath: "foo", Name: "bar", SubPath: "foo"}},
		},
		{
			name:    "volume device",
			cluster: []corev1.VolumeDevice{{DevicePath: "foo", Name: "foo"}},
			dc:      []corev1.VolumeDevice{{DevicePath: "foo", Name: "bar"}},
			want:    []corev1.VolumeDevice{{DevicePath: "foo", Name: "bar"}},
		},
		{
			name:    "racks",
			cluster: []cassdcapi.Rack{{Name: "foo", NodeAffinityLabels: map[string]string{"foo": "cluster", "bar": "cluster"}}},
			dc:      []cassdcapi.Rack{{Name: "foo", NodeAffinityLabels: map[string]string{"foo": "dc", "qix": "dc"}}},
			want:    []cassdcapi.Rack{{Name: "foo", NodeAffinityLabels: map[string]string{"foo": "dc", "bar": "cluster", "qix": "dc"}}},
		},
		{
			name:    "additional volumes",
			cluster: []cassdcapi.AdditionalVolumes{{Name: "foo", MountPath: "foo", PVCSpec: &corev1.PersistentVolumeClaimSpec{}}},
			dc:      []cassdcapi.AdditionalVolumes{{Name: "foo", MountPath: "bar"}},
			want:    []cassdcapi.AdditionalVolumes{{Name: "foo", MountPath: "bar", PVCSpec: &corev1.PersistentVolumeClaimSpec{}}},
		},
		// tests for the EnvVar type merger
		{
			name:    "env var zero vs non-zero",
			cluster: corev1.EnvVar{},
			dc:      corev1.EnvVar{Name: "foo", Value: "foo"},
			want:    corev1.EnvVar{Name: "foo", Value: "foo"},
		},
		{
			name:    "env var non-zero vs zero",
			cluster: corev1.EnvVar{Name: "foo", Value: "foo"},
			dc:      corev1.EnvVar{},
			want:    corev1.EnvVar{Name: "foo", Value: "foo"},
		},
		{
			name:    "env var non-zero vs non-zero",
			cluster: corev1.EnvVar{Name: "foo", Value: "foo"},
			dc:      corev1.EnvVar{Name: "foo", Value: "bar"},
			want:    corev1.EnvVar{Name: "foo", Value: "bar"},
		},
		{
			name:    "env var non-zero vs non-zero 2",
			cluster: corev1.EnvVar{Name: "foo", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}},
			dc:      corev1.EnvVar{Name: "foo", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{}}},
			want:    corev1.EnvVar{Name: "foo", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{}}},
		},
		{
			name:    "env var value vs valueFrom",
			cluster: corev1.EnvVar{Name: "foo", Value: "foo"},
			dc:      corev1.EnvVar{Name: "bar", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}},
			want:    corev1.EnvVar{Name: "bar", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}},
		},
		{
			name:    "env var value vs valueFrom 2",
			cluster: corev1.EnvVar{Name: "foo", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{}}},
			dc:      corev1.EnvVar{Name: "bar", Value: "foo"},
			want:    corev1.EnvVar{Name: "bar", Value: "foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeCRs(tt.cluster, tt.dc)
			assert.Equal(t, tt.want, got)
		})
	}
}
