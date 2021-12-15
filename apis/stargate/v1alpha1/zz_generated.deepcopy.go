//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2021.

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
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerImage) DeepCopyInto(out *ContainerImage) {
	*out = *in
	if in.PullSecretRef != nil {
		in, out := &in.PullSecretRef, &out.PullSecretRef
		*out = new(v1.LocalObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerImage.
func (in *ContainerImage) DeepCopy() *ContainerImage {
	if in == nil {
		return nil
	}
	out := new(ContainerImage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Stargate) DeepCopyInto(out *Stargate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Stargate.
func (in *Stargate) DeepCopy() *Stargate {
	if in == nil {
		return nil
	}
	out := new(Stargate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Stargate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateClusterTemplate) DeepCopyInto(out *StargateClusterTemplate) {
	*out = *in
	in.StargateTemplate.DeepCopyInto(&out.StargateTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateClusterTemplate.
func (in *StargateClusterTemplate) DeepCopy() *StargateClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(StargateClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateCondition) DeepCopyInto(out *StargateCondition) {
	*out = *in
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = (*in).DeepCopy()
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateCondition.
func (in *StargateCondition) DeepCopy() *StargateCondition {
	if in == nil {
		return nil
	}
	out := new(StargateCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateDatacenterTemplate) DeepCopyInto(out *StargateDatacenterTemplate) {
	*out = *in
	in.StargateClusterTemplate.DeepCopyInto(&out.StargateClusterTemplate)
	if in.Racks != nil {
		in, out := &in.Racks, &out.Racks
		*out = make([]StargateRackTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateDatacenterTemplate.
func (in *StargateDatacenterTemplate) DeepCopy() *StargateDatacenterTemplate {
	if in == nil {
		return nil
	}
	out := new(StargateDatacenterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateList) DeepCopyInto(out *StargateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Stargate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateList.
func (in *StargateList) DeepCopy() *StargateList {
	if in == nil {
		return nil
	}
	out := new(StargateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *StargateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateRackTemplate) DeepCopyInto(out *StargateRackTemplate) {
	*out = *in
	in.StargateTemplate.DeepCopyInto(&out.StargateTemplate)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateRackTemplate.
func (in *StargateRackTemplate) DeepCopy() *StargateRackTemplate {
	if in == nil {
		return nil
	}
	out := new(StargateRackTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateSpec) DeepCopyInto(out *StargateSpec) {
	*out = *in
	in.StargateDatacenterTemplate.DeepCopyInto(&out.StargateDatacenterTemplate)
	out.DatacenterRef = in.DatacenterRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateSpec.
func (in *StargateSpec) DeepCopy() *StargateSpec {
	if in == nil {
		return nil
	}
	out := new(StargateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateStatus) DeepCopyInto(out *StargateStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]StargateCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DeploymentRefs != nil {
		in, out := &in.DeploymentRefs, &out.DeploymentRefs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ServiceRef != nil {
		in, out := &in.ServiceRef, &out.ServiceRef
		*out = new(string)
		**out = **in
	}
	if in.ReadyReplicasRatio != nil {
		in, out := &in.ReadyReplicasRatio, &out.ReadyReplicasRatio
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateStatus.
func (in *StargateStatus) DeepCopy() *StargateStatus {
	if in == nil {
		return nil
	}
	out := new(StargateStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StargateTemplate) DeepCopyInto(out *StargateTemplate) {
	*out = *in
	if in.ContainerImage != nil {
		in, out := &in.ContainerImage, &out.ContainerImage
		*out = new(ContainerImage)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceAccount != nil {
		in, out := &in.ServiceAccount, &out.ServiceAccount
		*out = new(string)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.HeapSize != nil {
		in, out := &in.HeapSize, &out.HeapSize
		x := (*in).DeepCopy()
		*out = &x
	}
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
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]v1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(v1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.CassandraConfigMapRef != nil {
		in, out := &in.CassandraConfigMapRef, &out.CassandraConfigMapRef
		*out = new(v1.LocalObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StargateTemplate.
func (in *StargateTemplate) DeepCopy() *StargateTemplate {
	if in == nil {
		return nil
	}
	out := new(StargateTemplate)
	in.DeepCopyInto(out)
	return out
}
