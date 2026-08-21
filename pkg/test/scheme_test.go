/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package test

import (
	"testing"

	configapi "github.com/k8ssandra/k8ssandra-operator/apis/config/v1beta1"
	controlapi "github.com/k8ssandra/k8ssandra-operator/apis/control/v1alpha1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusaapi "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	reaperapi "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	replicationapi "github.com/k8ssandra/k8ssandra-operator/apis/replication/v1alpha1"
	stargateapi "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAPISchemesRegisterKnownTypes(t *testing.T) {
	tests := []struct {
		name         string
		groupVersion schema.GroupVersion
		addToScheme  func(*runtime.Scheme) error
		objects      []runtime.Object
	}{
		{
			name:         "config",
			groupVersion: configapi.GroupVersion,
			addToScheme:  configapi.AddToScheme,
			objects:      []runtime.Object{&configapi.ClientConfig{}, &configapi.ClientConfigList{}},
		},
		{
			name:         "control",
			groupVersion: controlapi.GroupVersion,
			addToScheme:  controlapi.AddToScheme,
			objects:      []runtime.Object{&controlapi.K8ssandraTask{}, &controlapi.K8ssandraTaskList{}},
		},
		{
			name:         "k8ssandra",
			groupVersion: k8ssandraapi.GroupVersion,
			addToScheme:  k8ssandraapi.AddToScheme,
			objects:      []runtime.Object{&k8ssandraapi.K8ssandraCluster{}, &k8ssandraapi.K8ssandraClusterList{}},
		},
		{
			name:         "medusa",
			groupVersion: medusaapi.GroupVersion,
			addToScheme:  medusaapi.AddToScheme,
			objects: []runtime.Object{
				&medusaapi.MedusaBackup{}, &medusaapi.MedusaBackupList{},
				&medusaapi.MedusaBackupJob{}, &medusaapi.MedusaBackupJobList{},
				&medusaapi.MedusaBackupSchedule{}, &medusaapi.MedusaBackupScheduleList{},
				&medusaapi.MedusaConfiguration{}, &medusaapi.MedusaConfigurationList{},
				&medusaapi.MedusaRestoreJob{}, &medusaapi.MedusaRestoreJobList{},
				&medusaapi.MedusaTask{}, &medusaapi.MedusaTaskList{},
			},
		},
		{
			name:         "reaper",
			groupVersion: reaperapi.GroupVersion,
			addToScheme:  reaperapi.AddToScheme,
			objects:      []runtime.Object{&reaperapi.Reaper{}, &reaperapi.ReaperList{}},
		},
		{
			name:         "replication",
			groupVersion: replicationapi.GroupVersion,
			addToScheme:  replicationapi.AddToScheme,
			objects:      []runtime.Object{&replicationapi.ReplicatedSecret{}, &replicationapi.ReplicatedSecretList{}},
		},
		{
			name:         "stargate",
			groupVersion: stargateapi.GroupVersion,
			addToScheme:  stargateapi.AddToScheme,
			objects:      []runtime.Object{&stargateapi.Stargate{}, &stargateapi.StargateList{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, tt.addToScheme(scheme))

			for _, object := range tt.objects {
				gvks, unversioned, err := scheme.ObjectKinds(object)
				require.NoError(t, err)
				require.False(t, unversioned)
				require.Len(t, gvks, 1)
				require.Equal(t, tt.groupVersion, gvks[0].GroupVersion())
			}
		})
	}
}
