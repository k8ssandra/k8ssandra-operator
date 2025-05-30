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

package meta

import ()

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraClusterMeta) DeepCopyInto(out *CassandraClusterMeta) {
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

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraClusterMeta.
func (in *CassandraClusterMeta) DeepCopy() *CassandraClusterMeta {
	if in == nil {
		return nil
	}
	out := new(CassandraClusterMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraDatacenterServicesMeta) DeepCopyInto(out *CassandraDatacenterServicesMeta) {
	*out = *in
	in.DatacenterService.DeepCopyInto(&out.DatacenterService)
	in.SeedService.DeepCopyInto(&out.SeedService)
	in.AllPodsService.DeepCopyInto(&out.AllPodsService)
	in.AdditionalSeedService.DeepCopyInto(&out.AdditionalSeedService)
	in.NodePortService.DeepCopyInto(&out.NodePortService)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraDatacenterServicesMeta.
func (in *CassandraDatacenterServicesMeta) DeepCopy() *CassandraDatacenterServicesMeta {
	if in == nil {
		return nil
	}
	out := new(CassandraDatacenterServicesMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceMeta) DeepCopyInto(out *ResourceMeta) {
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
	in.Service.DeepCopyInto(&out.Service)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceMeta.
func (in *ResourceMeta) DeepCopy() *ResourceMeta {
	if in == nil {
		return nil
	}
	out := new(ResourceMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Tags) DeepCopyInto(out *Tags) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Tags.
func (in *Tags) DeepCopy() *Tags {
	if in == nil {
		return nil
	}
	out := new(Tags)
	in.DeepCopyInto(out)
	return out
}
