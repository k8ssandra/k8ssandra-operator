//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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
	telemetryv1alpha1 "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/encryption"
	"github.com/k8ssandra/k8ssandra-operator/pkg/meta"
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AutoScheduling) DeepCopyInto(out *AutoScheduling) {
	*out = *in
	if in.ExcludedClusters != nil {
		in, out := &in.ExcludedClusters, &out.ExcludedClusters
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ExcludedKeyspaces != nil {
		in, out := &in.ExcludedKeyspaces, &out.ExcludedKeyspaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AutoScheduling.
func (in *AutoScheduling) DeepCopy() *AutoScheduling {
	if in == nil {
		return nil
	}
	out := new(AutoScheduling)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraDatacenterRef) DeepCopyInto(out *CassandraDatacenterRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraDatacenterRef.
func (in *CassandraDatacenterRef) DeepCopy() *CassandraDatacenterRef {
	if in == nil {
		return nil
	}
	out := new(CassandraDatacenterRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Reaper) DeepCopyInto(out *Reaper) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Reaper.
func (in *Reaper) DeepCopy() *Reaper {
	if in == nil {
		return nil
	}
	out := new(Reaper)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Reaper) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReaperClusterTemplate) DeepCopyInto(out *ReaperClusterTemplate) {
	*out = *in
	in.ReaperTemplate.DeepCopyInto(&out.ReaperTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReaperClusterTemplate.
func (in *ReaperClusterTemplate) DeepCopy() *ReaperClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(ReaperClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReaperCondition) DeepCopyInto(out *ReaperCondition) {
	*out = *in
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReaperCondition.
func (in *ReaperCondition) DeepCopy() *ReaperCondition {
	if in == nil {
		return nil
	}
	out := new(ReaperCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReaperList) DeepCopyInto(out *ReaperList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Reaper, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReaperList.
func (in *ReaperList) DeepCopy() *ReaperList {
	if in == nil {
		return nil
	}
	out := new(ReaperList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ReaperList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReaperSpec) DeepCopyInto(out *ReaperSpec) {
	*out = *in
	in.ReaperTemplate.DeepCopyInto(&out.ReaperTemplate)
	out.DatacenterRef = in.DatacenterRef
	if in.ClientEncryptionStores != nil {
		in, out := &in.ClientEncryptionStores, &out.ClientEncryptionStores
		*out = new(encryption.Stores)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReaperSpec.
func (in *ReaperSpec) DeepCopy() *ReaperSpec {
	if in == nil {
		return nil
	}
	out := new(ReaperSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReaperStatus) DeepCopyInto(out *ReaperStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]ReaperCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReaperStatus.
func (in *ReaperStatus) DeepCopy() *ReaperStatus {
	if in == nil {
		return nil
	}
	out := new(ReaperStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ReaperTemplate) DeepCopyInto(out *ReaperTemplate) {
	*out = *in
	out.CassandraUserSecretRef = in.CassandraUserSecretRef
	out.JmxUserSecretRef = in.JmxUserSecretRef
	out.UiUserSecretRef = in.UiUserSecretRef
	in.AutoScheduling.DeepCopyInto(&out.AutoScheduling)
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(v1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(v1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(v1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.PodSecurityContext != nil {
		in, out := &in.PodSecurityContext, &out.PodSecurityContext
		*out = new(v1.PodSecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(v1.SecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.InitContainerSecurityContext != nil {
		in, out := &in.InitContainerSecurityContext, &out.InitContainerSecurityContext
		*out = new(v1.SecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.HeapSize != nil {
		in, out := &in.HeapSize, &out.HeapSize
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.InitContainerResources != nil {
		in, out := &in.InitContainerResources, &out.InitContainerResources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.Telemetry != nil {
		in, out := &in.Telemetry, &out.Telemetry
		*out = new(telemetryv1alpha1.TelemetrySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.ResourceMeta != nil {
		in, out := &in.ResourceMeta, &out.ResourceMeta
		*out = new(meta.ResourceMeta)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ReaperTemplate.
func (in *ReaperTemplate) DeepCopy() *ReaperTemplate {
	if in == nil {
		return nil
	}
	out := new(ReaperTemplate)
	in.DeepCopyInto(out)
	return out
}
