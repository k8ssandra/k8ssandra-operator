//go:build !ignore_autogenerated

/*
Copyright 2022.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusav1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	reaperv1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/reaper/v1alpha1"
	stargatev1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/stargate/v1alpha1"
	telemetryv1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuditLogOptions) DeepCopyInto(out *AuditLogOptions) {
	*out = *in
	if in.Logger != nil {
		in, out := &in.Logger, &out.Logger
		*out = new(ParameterizedClass)
		(*in).DeepCopyInto(*out)
	}
	if in.IncludedKeyspaces != nil {
		in, out := &in.IncludedKeyspaces, &out.IncludedKeyspaces
		*out = new(string)
		**out = **in
	}
	if in.ExcludedKeyspaces != nil {
		in, out := &in.ExcludedKeyspaces, &out.ExcludedKeyspaces
		*out = new(string)
		**out = **in
	}
	if in.IncludedCategories != nil {
		in, out := &in.IncludedCategories, &out.IncludedCategories
		*out = new(string)
		**out = **in
	}
	if in.ExcludedCategories != nil {
		in, out := &in.ExcludedCategories, &out.ExcludedCategories
		*out = new(string)
		**out = **in
	}
	if in.IncludedUsers != nil {
		in, out := &in.IncludedUsers, &out.IncludedUsers
		*out = new(string)
		**out = **in
	}
	if in.ExcludedUsers != nil {
		in, out := &in.ExcludedUsers, &out.ExcludedUsers
		*out = new(string)
		**out = **in
	}
	if in.RollCycle != nil {
		in, out := &in.RollCycle, &out.RollCycle
		*out = new(string)
		**out = **in
	}
	if in.Block != nil {
		in, out := &in.Block, &out.Block
		*out = new(bool)
		**out = **in
	}
	if in.MaxQueueWeight != nil {
		in, out := &in.MaxQueueWeight, &out.MaxQueueWeight
		*out = new(int)
		**out = **in
	}
	if in.MaxLogSize != nil {
		in, out := &in.MaxLogSize, &out.MaxLogSize
		*out = new(int)
		**out = **in
	}
	if in.ArchiveCommand != nil {
		in, out := &in.ArchiveCommand, &out.ArchiveCommand
		*out = new(string)
		**out = **in
	}
	if in.MaxArchiveRetries != nil {
		in, out := &in.MaxArchiveRetries, &out.MaxArchiveRetries
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuditLogOptions.
func (in *AuditLogOptions) DeepCopy() *AuditLogOptions {
	if in == nil {
		return nil
	}
	out := new(AuditLogOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraClusterTemplate) DeepCopyInto(out *CassandraClusterTemplate) {
	*out = *in
	in.DatacenterOptions.DeepCopyInto(&out.DatacenterOptions)
	in.Meta.DeepCopyInto(&out.Meta)
	out.SuperuserSecretRef = in.SuperuserSecretRef
	if in.Datacenters != nil {
		in, out := &in.Datacenters, &out.Datacenters
		*out = make([]CassandraDatacenterTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AdditionalSeeds != nil {
		in, out := &in.AdditionalSeeds, &out.AdditionalSeeds
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ServerEncryptionStores != nil {
		in, out := &in.ServerEncryptionStores, &out.ServerEncryptionStores
		*out = new(encryption.Stores)
		(*in).DeepCopyInto(*out)
	}
	if in.ClientEncryptionStores != nil {
		in, out := &in.ClientEncryptionStores, &out.ClientEncryptionStores
		*out = new(encryption.Stores)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraClusterTemplate.
func (in *CassandraClusterTemplate) DeepCopy() *CassandraClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(CassandraClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraConfig) DeepCopyInto(out *CassandraConfig) {
	*out = *in
	in.CassandraYaml.DeepCopyInto(&out.CassandraYaml)
	in.JvmOptions.DeepCopyInto(&out.JvmOptions)
	in.DseYaml.DeepCopyInto(&out.DseYaml)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraConfig.
func (in *CassandraConfig) DeepCopy() *CassandraConfig {
	if in == nil {
		return nil
	}
	out := new(CassandraConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraDatacenterTemplate) DeepCopyInto(out *CassandraDatacenterTemplate) {
	*out = *in
	in.Meta.DeepCopyInto(&out.Meta)
	in.DatacenterOptions.DeepCopyInto(&out.DatacenterOptions)
	if in.Stargate != nil {
		in, out := &in.Stargate, &out.Stargate
		*out = new(stargatev1alpha1.StargateDatacenterTemplate)
		(*in).DeepCopyInto(*out)
	}
	out.PerNodeConfigMapRef = in.PerNodeConfigMapRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraDatacenterTemplate.
func (in *CassandraDatacenterTemplate) DeepCopy() *CassandraDatacenterTemplate {
	if in == nil {
		return nil
	}
	out := new(CassandraDatacenterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DatacenterOptions) DeepCopyInto(out *DatacenterOptions) {
	*out = *in
	if in.CassandraConfig != nil {
		in, out := &in.CassandraConfig, &out.CassandraConfig
		*out = new(CassandraConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.StorageConfig != nil {
		in, out := &in.StorageConfig, &out.StorageConfig
		*out = new(v1beta1.StorageConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Networking != nil {
		in, out := &in.Networking, &out.Networking
		*out = new(NetworkingConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.Racks != nil {
		in, out := &in.Racks, &out.Racks
		*out = make([]v1beta1.Rack, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.JmxInitContainerImage != nil {
		in, out := &in.JmxInitContainerImage, &out.JmxInitContainerImage
		*out = new(images.Image)
		(*in).DeepCopyInto(*out)
	}
	if in.SoftPodAntiAffinity != nil {
		in, out := &in.SoftPodAntiAffinity, &out.SoftPodAntiAffinity
		*out = new(bool)
		**out = **in
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.MgmtAPIHeap != nil {
		in, out := &in.MgmtAPIHeap, &out.MgmtAPIHeap
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.Telemetry != nil {
		in, out := &in.Telemetry, &out.Telemetry
		*out = new(telemetryv1alpha1.TelemetrySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.CDC != nil {
		in, out := &in.CDC, &out.CDC
		*out = new(v1beta1.CDCConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Containers != nil {
		in, out := &in.Containers, &out.Containers
		*out = make([]v1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.InitContainers != nil {
		in, out := &in.InitContainers, &out.InitContainers
		*out = make([]v1.Container, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ExtraVolumes != nil {
		in, out := &in.ExtraVolumes, &out.ExtraVolumes
		*out = new(K8ssandraVolumes)
		(*in).DeepCopyInto(*out)
	}
	if in.DseWorkloads != nil {
		in, out := &in.DseWorkloads, &out.DseWorkloads
		*out = new(v1beta1.DseWorkloads)
		**out = **in
	}
	if in.PodSecurityContext != nil {
		in, out := &in.PodSecurityContext, &out.PodSecurityContext
		*out = new(v1.PodSecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.ManagementApiAuth != nil {
		in, out := &in.ManagementApiAuth, &out.ManagementApiAuth
		*out = new(v1beta1.ManagementApiAuthConfig)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DatacenterOptions.
func (in *DatacenterOptions) DeepCopy() *DatacenterOptions {
	if in == nil {
		return nil
	}
	out := new(DatacenterOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EmbeddedObjectMeta) DeepCopyInto(out *EmbeddedObjectMeta) {
	*out = *in
	in.Tags.DeepCopyInto(&out.Tags)
	if in.CommonLabels != nil {
		in, out := &in.CommonLabels, &out.CommonLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.CommonAnnotations != nil {
		in, out := &in.CommonAnnotations, &out.CommonAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.Pods.DeepCopyInto(&out.Pods)
	in.ServiceConfig.DeepCopyInto(&out.ServiceConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EmbeddedObjectMeta.
func (in *EmbeddedObjectMeta) DeepCopy() *EmbeddedObjectMeta {
	if in == nil {
		return nil
	}
	out := new(EmbeddedObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FullQueryLoggerOptions) DeepCopyInto(out *FullQueryLoggerOptions) {
	*out = *in
	if in.ArchiveCommand != nil {
		in, out := &in.ArchiveCommand, &out.ArchiveCommand
		*out = new(string)
		**out = **in
	}
	if in.RollCycle != nil {
		in, out := &in.RollCycle, &out.RollCycle
		*out = new(string)
		**out = **in
	}
	if in.Block != nil {
		in, out := &in.Block, &out.Block
		*out = new(bool)
		**out = **in
	}
	if in.MaxQueueWeight != nil {
		in, out := &in.MaxQueueWeight, &out.MaxQueueWeight
		*out = new(int)
		**out = **in
	}
	if in.MaxLogSize != nil {
		in, out := &in.MaxLogSize, &out.MaxLogSize
		*out = new(int)
		**out = **in
	}
	if in.MaxArchiveRetries != nil {
		in, out := &in.MaxArchiveRetries, &out.MaxArchiveRetries
		*out = new(int)
		**out = **in
	}
	if in.LogDir != nil {
		in, out := &in.LogDir, &out.LogDir
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FullQueryLoggerOptions.
func (in *FullQueryLoggerOptions) DeepCopy() *FullQueryLoggerOptions {
	if in == nil {
		return nil
	}
	out := new(FullQueryLoggerOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *JvmOptions) DeepCopyInto(out *JvmOptions) {
	*out = *in
	if in.HeapSize != nil {
		in, out := &in.HeapSize, &out.HeapSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.InitialHeapSize != nil {
		in, out := &in.InitialHeapSize, &out.InitialHeapSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.MaxHeapSize != nil {
		in, out := &in.MaxHeapSize, &out.MaxHeapSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.EnableAssertions != nil {
		in, out := &in.EnableAssertions, &out.EnableAssertions
		*out = new(bool)
		**out = **in
	}
	if in.EnableThreadPriorities != nil {
		in, out := &in.EnableThreadPriorities, &out.EnableThreadPriorities
		*out = new(bool)
		**out = **in
	}
	if in.EnableNonRootThreadPriority != nil {
		in, out := &in.EnableNonRootThreadPriority, &out.EnableNonRootThreadPriority
		*out = new(bool)
		**out = **in
	}
	if in.HeapDumpOnOutOfMemoryError != nil {
		in, out := &in.HeapDumpOnOutOfMemoryError, &out.HeapDumpOnOutOfMemoryError
		*out = new(bool)
		**out = **in
	}
	if in.PerThreadStackSize != nil {
		in, out := &in.PerThreadStackSize, &out.PerThreadStackSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.StringTableSize != nil {
		in, out := &in.StringTableSize, &out.StringTableSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.AlwaysPreTouch != nil {
		in, out := &in.AlwaysPreTouch, &out.AlwaysPreTouch
		*out = new(bool)
		**out = **in
	}
	if in.DisableBiasedLocking != nil {
		in, out := &in.DisableBiasedLocking, &out.DisableBiasedLocking
		*out = new(bool)
		**out = **in
	}
	if in.UseTlab != nil {
		in, out := &in.UseTlab, &out.UseTlab
		*out = new(bool)
		**out = **in
	}
	if in.ResizeTlab != nil {
		in, out := &in.ResizeTlab, &out.ResizeTlab
		*out = new(bool)
		**out = **in
	}
	if in.DisablePerfSharedMem != nil {
		in, out := &in.DisablePerfSharedMem, &out.DisablePerfSharedMem
		*out = new(bool)
		**out = **in
	}
	if in.PreferIpv4 != nil {
		in, out := &in.PreferIpv4, &out.PreferIpv4
		*out = new(bool)
		**out = **in
	}
	if in.UseNuma != nil {
		in, out := &in.UseNuma, &out.UseNuma
		*out = new(bool)
		**out = **in
	}
	if in.PrintHeapHistogramOnOutOfMemoryError != nil {
		in, out := &in.PrintHeapHistogramOnOutOfMemoryError, &out.PrintHeapHistogramOnOutOfMemoryError
		*out = new(bool)
		**out = **in
	}
	if in.ExitOnOutOfMemoryError != nil {
		in, out := &in.ExitOnOutOfMemoryError, &out.ExitOnOutOfMemoryError
		*out = new(bool)
		**out = **in
	}
	if in.CrashOnOutOfMemoryError != nil {
		in, out := &in.CrashOnOutOfMemoryError, &out.CrashOnOutOfMemoryError
		*out = new(bool)
		**out = **in
	}
	if in.GuaranteedSafepointIntervalMs != nil {
		in, out := &in.GuaranteedSafepointIntervalMs, &out.GuaranteedSafepointIntervalMs
		*out = new(int)
		**out = **in
	}
	if in.NettyEventloopMaxPendingTasks != nil {
		in, out := &in.NettyEventloopMaxPendingTasks, &out.NettyEventloopMaxPendingTasks
		*out = new(int)
		**out = **in
	}
	if in.NettyTryReflectionSetAccessible != nil {
		in, out := &in.NettyTryReflectionSetAccessible, &out.NettyTryReflectionSetAccessible
		*out = new(bool)
		**out = **in
	}
	if in.NioMaxCachedBufferSize != nil {
		in, out := &in.NioMaxCachedBufferSize, &out.NioMaxCachedBufferSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.NioAlignDirectMemory != nil {
		in, out := &in.NioAlignDirectMemory, &out.NioAlignDirectMemory
		*out = new(bool)
		**out = **in
	}
	if in.JdkAllowAttachSelf != nil {
		in, out := &in.JdkAllowAttachSelf, &out.JdkAllowAttachSelf
		*out = new(bool)
		**out = **in
	}
	if in.AvailableProcessors != nil {
		in, out := &in.AvailableProcessors, &out.AvailableProcessors
		*out = new(int)
		**out = **in
	}
	if in.MetricsReporterConfigFile != nil {
		in, out := &in.MetricsReporterConfigFile, &out.MetricsReporterConfigFile
		*out = new(string)
		**out = **in
	}
	if in.RingDelayMs != nil {
		in, out := &in.RingDelayMs, &out.RingDelayMs
		*out = new(int)
		**out = **in
	}
	if in.TriggersDirectory != nil {
		in, out := &in.TriggersDirectory, &out.TriggersDirectory
		*out = new(string)
		**out = **in
	}
	if in.WriteSurvey != nil {
		in, out := &in.WriteSurvey, &out.WriteSurvey
		*out = new(bool)
		**out = **in
	}
	if in.DisableAuthCachesRemoteConfiguration != nil {
		in, out := &in.DisableAuthCachesRemoteConfiguration, &out.DisableAuthCachesRemoteConfiguration
		*out = new(bool)
		**out = **in
	}
	if in.ForceDefaultIndexingPageSize != nil {
		in, out := &in.ForceDefaultIndexingPageSize, &out.ForceDefaultIndexingPageSize
		*out = new(bool)
		**out = **in
	}
	if in.Force30ProtocolVersion != nil {
		in, out := &in.Force30ProtocolVersion, &out.Force30ProtocolVersion
		*out = new(bool)
		**out = **in
	}
	if in.ExpirationDateOverflowPolicy != nil {
		in, out := &in.ExpirationDateOverflowPolicy, &out.ExpirationDateOverflowPolicy
		*out = new(string)
		**out = **in
	}
	if in.MaxHintTtlSeconds != nil {
		in, out := &in.MaxHintTtlSeconds, &out.MaxHintTtlSeconds
		*out = new(int)
		**out = **in
	}
	if in.GarbageCollector != nil {
		in, out := &in.GarbageCollector, &out.GarbageCollector
		*out = new(string)
		**out = **in
	}
	if in.HeapNewGenSize != nil {
		in, out := &in.HeapNewGenSize, &out.HeapNewGenSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.CmsHeapSizeYoungGeneration != nil {
		in, out := &in.CmsHeapSizeYoungGeneration, &out.CmsHeapSizeYoungGeneration
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.CmsSurvivorRatio != nil {
		in, out := &in.CmsSurvivorRatio, &out.CmsSurvivorRatio
		*out = new(int)
		**out = **in
	}
	if in.CmsMaxTenuringThreshold != nil {
		in, out := &in.CmsMaxTenuringThreshold, &out.CmsMaxTenuringThreshold
		*out = new(int)
		**out = **in
	}
	if in.CmsInitiatingOccupancyFraction != nil {
		in, out := &in.CmsInitiatingOccupancyFraction, &out.CmsInitiatingOccupancyFraction
		*out = new(int)
		**out = **in
	}
	if in.CmsWaitDurationMs != nil {
		in, out := &in.CmsWaitDurationMs, &out.CmsWaitDurationMs
		*out = new(int)
		**out = **in
	}
	if in.G1RSetUpdatingPauseTimePercent != nil {
		in, out := &in.G1RSetUpdatingPauseTimePercent, &out.G1RSetUpdatingPauseTimePercent
		*out = new(int)
		**out = **in
	}
	if in.G1MaxGcPauseMs != nil {
		in, out := &in.G1MaxGcPauseMs, &out.G1MaxGcPauseMs
		*out = new(int)
		**out = **in
	}
	if in.G1InitiatingHeapOccupancyPercent != nil {
		in, out := &in.G1InitiatingHeapOccupancyPercent, &out.G1InitiatingHeapOccupancyPercent
		*out = new(int)
		**out = **in
	}
	if in.G1ParallelGcThreads != nil {
		in, out := &in.G1ParallelGcThreads, &out.G1ParallelGcThreads
		*out = new(int)
		**out = **in
	}
	if in.G1ConcGcThreads != nil {
		in, out := &in.G1ConcGcThreads, &out.G1ConcGcThreads
		*out = new(int)
		**out = **in
	}
	if in.PrintDetails != nil {
		in, out := &in.PrintDetails, &out.PrintDetails
		*out = new(bool)
		**out = **in
	}
	if in.PrintDateStamps != nil {
		in, out := &in.PrintDateStamps, &out.PrintDateStamps
		*out = new(bool)
		**out = **in
	}
	if in.PrintHeap != nil {
		in, out := &in.PrintHeap, &out.PrintHeap
		*out = new(bool)
		**out = **in
	}
	if in.PrintTenuringDistribution != nil {
		in, out := &in.PrintTenuringDistribution, &out.PrintTenuringDistribution
		*out = new(bool)
		**out = **in
	}
	if in.PrintApplicationStoppedTime != nil {
		in, out := &in.PrintApplicationStoppedTime, &out.PrintApplicationStoppedTime
		*out = new(bool)
		**out = **in
	}
	if in.PrintPromotionFailure != nil {
		in, out := &in.PrintPromotionFailure, &out.PrintPromotionFailure
		*out = new(bool)
		**out = **in
	}
	if in.PrintFlssStatistics != nil {
		in, out := &in.PrintFlssStatistics, &out.PrintFlssStatistics
		*out = new(bool)
		**out = **in
	}
	if in.UseLogFile != nil {
		in, out := &in.UseLogFile, &out.UseLogFile
		*out = new(bool)
		**out = **in
	}
	if in.UseLogFileRotation != nil {
		in, out := &in.UseLogFileRotation, &out.UseLogFileRotation
		*out = new(bool)
		**out = **in
	}
	if in.NumberOfLogFiles != nil {
		in, out := &in.NumberOfLogFiles, &out.NumberOfLogFiles
		*out = new(int)
		**out = **in
	}
	if in.LogFileSize != nil {
		in, out := &in.LogFileSize, &out.LogFileSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.JmxPort != nil {
		in, out := &in.JmxPort, &out.JmxPort
		*out = new(int)
		**out = **in
	}
	if in.JmxConnectionType != nil {
		in, out := &in.JmxConnectionType, &out.JmxConnectionType
		*out = new(string)
		**out = **in
	}
	if in.JmxRemoteSsl != nil {
		in, out := &in.JmxRemoteSsl, &out.JmxRemoteSsl
		*out = new(bool)
		**out = **in
	}
	if in.JmxRemoteSslOpts != nil {
		in, out := &in.JmxRemoteSslOpts, &out.JmxRemoteSslOpts
		*out = new(string)
		**out = **in
	}
	if in.JmxRemoteSslRequireClientAuth != nil {
		in, out := &in.JmxRemoteSslRequireClientAuth, &out.JmxRemoteSslRequireClientAuth
		*out = new(bool)
		**out = **in
	}
	if in.UnlockCommercialFeatures != nil {
		in, out := &in.UnlockCommercialFeatures, &out.UnlockCommercialFeatures
		*out = new(bool)
		**out = **in
	}
	if in.EnableFlightRecorder != nil {
		in, out := &in.EnableFlightRecorder, &out.EnableFlightRecorder
		*out = new(bool)
		**out = **in
	}
	if in.ListenForRemoteDebuggers != nil {
		in, out := &in.ListenForRemoteDebuggers, &out.ListenForRemoteDebuggers
		*out = new(bool)
		**out = **in
	}
	if in.DisableContendedAnnotations != nil {
		in, out := &in.DisableContendedAnnotations, &out.DisableContendedAnnotations
		*out = new(bool)
		**out = **in
	}
	if in.DebugNonSafepoints != nil {
		in, out := &in.DebugNonSafepoints, &out.DebugNonSafepoints
		*out = new(bool)
		**out = **in
	}
	if in.UnlockDiagnosticVmOptions != nil {
		in, out := &in.UnlockDiagnosticVmOptions, &out.UnlockDiagnosticVmOptions
		*out = new(bool)
		**out = **in
	}
	if in.LogCompilation != nil {
		in, out := &in.LogCompilation, &out.LogCompilation
		*out = new(bool)
		**out = **in
	}
	if in.PreserveFramePointer != nil {
		in, out := &in.PreserveFramePointer, &out.PreserveFramePointer
		*out = new(bool)
		**out = **in
	}
	if in.AdditionalOptions != nil {
		in, out := &in.AdditionalOptions, &out.AdditionalOptions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AdditionalJvm11ServerOptions != nil {
		in, out := &in.AdditionalJvm11ServerOptions, &out.AdditionalJvm11ServerOptions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AdditionalJvm8ServerOptions != nil {
		in, out := &in.AdditionalJvm8ServerOptions, &out.AdditionalJvm8ServerOptions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AdditionalJvmServerOptions != nil {
		in, out := &in.AdditionalJvmServerOptions, &out.AdditionalJvmServerOptions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new JvmOptions.
func (in *JvmOptions) DeepCopy() *JvmOptions {
	if in == nil {
		return nil
	}
	out := new(JvmOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraCluster) DeepCopyInto(out *K8ssandraCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraCluster.
func (in *K8ssandraCluster) DeepCopy() *K8ssandraCluster {
	if in == nil {
		return nil
	}
	out := new(K8ssandraCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *K8ssandraCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraClusterCondition) DeepCopyInto(out *K8ssandraClusterCondition) {
	*out = *in
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraClusterCondition.
func (in *K8ssandraClusterCondition) DeepCopy() *K8ssandraClusterCondition {
	if in == nil {
		return nil
	}
	out := new(K8ssandraClusterCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraClusterList) DeepCopyInto(out *K8ssandraClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]K8ssandraCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraClusterList.
func (in *K8ssandraClusterList) DeepCopy() *K8ssandraClusterList {
	if in == nil {
		return nil
	}
	out := new(K8ssandraClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *K8ssandraClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraClusterSpec) DeepCopyInto(out *K8ssandraClusterSpec) {
	*out = *in
	if in.Auth != nil {
		in, out := &in.Auth, &out.Auth
		*out = new(bool)
		**out = **in
	}
	if in.Cassandra != nil {
		in, out := &in.Cassandra, &out.Cassandra
		*out = new(CassandraClusterTemplate)
		(*in).DeepCopyInto(*out)
	}
	if in.Stargate != nil {
		in, out := &in.Stargate, &out.Stargate
		*out = new(stargatev1alpha1.StargateClusterTemplate)
		(*in).DeepCopyInto(*out)
	}
	if in.Reaper != nil {
		in, out := &in.Reaper, &out.Reaper
		*out = new(reaperv1alpha1.ReaperClusterTemplate)
		(*in).DeepCopyInto(*out)
	}
	if in.Medusa != nil {
		in, out := &in.Medusa, &out.Medusa
		*out = new(medusav1alpha1.MedusaClusterTemplate)
		(*in).DeepCopyInto(*out)
	}
	if in.ExternalDatacenters != nil {
		in, out := &in.ExternalDatacenters, &out.ExternalDatacenters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraClusterSpec.
func (in *K8ssandraClusterSpec) DeepCopy() *K8ssandraClusterSpec {
	if in == nil {
		return nil
	}
	out := new(K8ssandraClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraClusterStatus) DeepCopyInto(out *K8ssandraClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]K8ssandraClusterCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Datacenters != nil {
		in, out := &in.Datacenters, &out.Datacenters
		*out = make(map[string]K8ssandraStatus, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraClusterStatus.
func (in *K8ssandraClusterStatus) DeepCopy() *K8ssandraClusterStatus {
	if in == nil {
		return nil
	}
	out := new(K8ssandraClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraStatus) DeepCopyInto(out *K8ssandraStatus) {
	*out = *in
	if in.Cassandra != nil {
		in, out := &in.Cassandra, &out.Cassandra
		*out = new(v1beta1.CassandraDatacenterStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.Stargate != nil {
		in, out := &in.Stargate, &out.Stargate
		*out = new(stargatev1alpha1.StargateStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.Reaper != nil {
		in, out := &in.Reaper, &out.Reaper
		*out = new(reaperv1alpha1.ReaperStatus)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraStatus.
func (in *K8ssandraStatus) DeepCopy() *K8ssandraStatus {
	if in == nil {
		return nil
	}
	out := new(K8ssandraStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *K8ssandraVolumes) DeepCopyInto(out *K8ssandraVolumes) {
	*out = *in
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]v1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.PVCs != nil {
		in, out := &in.PVCs, &out.PVCs
		*out = make([]v1beta1.AdditionalVolumes, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new K8ssandraVolumes.
func (in *K8ssandraVolumes) DeepCopy() *K8ssandraVolumes {
	if in == nil {
		return nil
	}
	out := new(K8ssandraVolumes)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkingConfig) DeepCopyInto(out *NetworkingConfig) {
	*out = *in
	if in.NodePort != nil {
		in, out := &in.NodePort, &out.NodePort
		*out = new(v1beta1.NodePortConfig)
		**out = **in
	}
	if in.HostNetwork != nil {
		in, out := &in.HostNetwork, &out.HostNetwork
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkingConfig.
func (in *NetworkingConfig) DeepCopy() *NetworkingConfig {
	if in == nil {
		return nil
	}
	out := new(NetworkingConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ParameterizedClass) DeepCopyInto(out *ParameterizedClass) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = new(map[string]string)
		if **in != nil {
			in, out := *in, *out
			*out = make(map[string]string, len(*in))
			for key, val := range *in {
				(*out)[key] = val
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ParameterizedClass.
func (in *ParameterizedClass) DeepCopy() *ParameterizedClass {
	if in == nil {
		return nil
	}
	out := new(ParameterizedClass)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReplicaFilteringProtectionOptions) DeepCopyInto(out *ReplicaFilteringProtectionOptions) {
	*out = *in
	if in.CachedRowsWarnThreshold != nil {
		in, out := &in.CachedRowsWarnThreshold, &out.CachedRowsWarnThreshold
		*out = new(int)
		**out = **in
	}
	if in.CachedRowsFailThreshold != nil {
		in, out := &in.CachedRowsFailThreshold, &out.CachedRowsFailThreshold
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReplicaFilteringProtectionOptions.
func (in *ReplicaFilteringProtectionOptions) DeepCopy() *ReplicaFilteringProtectionOptions {
	if in == nil {
		return nil
	}
	out := new(ReplicaFilteringProtectionOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RequestSchedulerOptions) DeepCopyInto(out *RequestSchedulerOptions) {
	*out = *in
	if in.ThrottleLimit != nil {
		in, out := &in.ThrottleLimit, &out.ThrottleLimit
		*out = new(int)
		**out = **in
	}
	if in.DefaultWeight != nil {
		in, out := &in.DefaultWeight, &out.DefaultWeight
		*out = new(int)
		**out = **in
	}
	if in.Weights != nil {
		in, out := &in.Weights, &out.Weights
		*out = new(map[string]int)
		if **in != nil {
			in, out := *in, *out
			*out = make(map[string]int, len(*in))
			for key, val := range *in {
				(*out)[key] = val
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RequestSchedulerOptions.
func (in *RequestSchedulerOptions) DeepCopy() *RequestSchedulerOptions {
	if in == nil {
		return nil
	}
	out := new(RequestSchedulerOptions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubnetGroups) DeepCopyInto(out *SubnetGroups) {
	*out = *in
	if in.Subnets != nil {
		in, out := &in.Subnets, &out.Subnets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SubnetGroups.
func (in *SubnetGroups) DeepCopy() *SubnetGroups {
	if in == nil {
		return nil
	}
	out := new(SubnetGroups)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TrackWarnings) DeepCopyInto(out *TrackWarnings) {
	*out = *in
	if in.CoordinatorReadSize != nil {
		in, out := &in.CoordinatorReadSize, &out.CoordinatorReadSize
		*out = new(int)
		**out = **in
	}
	if in.LocalReadSize != nil {
		in, out := &in.LocalReadSize, &out.LocalReadSize
		*out = new(int)
		**out = **in
	}
	if in.RowIndexSize != nil {
		in, out := &in.RowIndexSize, &out.RowIndexSize
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrackWarnings.
func (in *TrackWarnings) DeepCopy() *TrackWarnings {
	if in == nil {
		return nil
	}
	out := new(TrackWarnings)
	in.DeepCopyInto(out)
	return out
}
