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
	"github.com/k8ssandra/k8ssandra-operator/pkg/images"
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraBackup) DeepCopyInto(out *CassandraBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraBackup.
func (in *CassandraBackup) DeepCopy() *CassandraBackup {
	if in == nil {
		return nil
	}
	out := new(CassandraBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CassandraBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraBackupList) DeepCopyInto(out *CassandraBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CassandraBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraBackupList.
func (in *CassandraBackupList) DeepCopy() *CassandraBackupList {
	if in == nil {
		return nil
	}
	out := new(CassandraBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CassandraBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraBackupSpec) DeepCopyInto(out *CassandraBackupSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraBackupSpec.
func (in *CassandraBackupSpec) DeepCopy() *CassandraBackupSpec {
	if in == nil {
		return nil
	}
	out := new(CassandraBackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraBackupStatus) DeepCopyInto(out *CassandraBackupStatus) {
	*out = *in
	if in.CassdcTemplateSpec != nil {
		in, out := &in.CassdcTemplateSpec, &out.CassdcTemplateSpec
		*out = new(CassandraDatacenterTemplateSpec)
		(*in).DeepCopyInto(*out)
	}
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
	if in.InProgress != nil {
		in, out := &in.InProgress, &out.InProgress
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Finished != nil {
		in, out := &in.Finished, &out.Finished
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Failed != nil {
		in, out := &in.Failed, &out.Failed
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraBackupStatus.
func (in *CassandraBackupStatus) DeepCopy() *CassandraBackupStatus {
	if in == nil {
		return nil
	}
	out := new(CassandraBackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraDatacenterConfig) DeepCopyInto(out *CassandraDatacenterConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraDatacenterConfig.
func (in *CassandraDatacenterConfig) DeepCopy() *CassandraDatacenterConfig {
	if in == nil {
		return nil
	}
	out := new(CassandraDatacenterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraDatacenterTemplateSpec) DeepCopyInto(out *CassandraDatacenterTemplateSpec) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraDatacenterTemplateSpec.
func (in *CassandraDatacenterTemplateSpec) DeepCopy() *CassandraDatacenterTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(CassandraDatacenterTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraRestore) DeepCopyInto(out *CassandraRestore) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraRestore.
func (in *CassandraRestore) DeepCopy() *CassandraRestore {
	if in == nil {
		return nil
	}
	out := new(CassandraRestore)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CassandraRestore) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraRestoreList) DeepCopyInto(out *CassandraRestoreList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CassandraRestore, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraRestoreList.
func (in *CassandraRestoreList) DeepCopy() *CassandraRestoreList {
	if in == nil {
		return nil
	}
	out := new(CassandraRestoreList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CassandraRestoreList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraRestoreSpec) DeepCopyInto(out *CassandraRestoreSpec) {
	*out = *in
	out.CassandraDatacenter = in.CassandraDatacenter
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraRestoreSpec.
func (in *CassandraRestoreSpec) DeepCopy() *CassandraRestoreSpec {
	if in == nil {
		return nil
	}
	out := new(CassandraRestoreSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraRestoreStatus) DeepCopyInto(out *CassandraRestoreStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
	in.DatacenterStopped.DeepCopyInto(&out.DatacenterStopped)
	if in.InProgress != nil {
		in, out := &in.InProgress, &out.InProgress
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Finished != nil {
		in, out := &in.Finished, &out.Finished
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Failed != nil {
		in, out := &in.Failed, &out.Failed
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraRestoreStatus.
func (in *CassandraRestoreStatus) DeepCopy() *CassandraRestoreStatus {
	if in == nil {
		return nil
	}
	out := new(CassandraRestoreStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackup) DeepCopyInto(out *MedusaBackup) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackup.
func (in *MedusaBackup) DeepCopy() *MedusaBackup {
	if in == nil {
		return nil
	}
	out := new(MedusaBackup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaBackup) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupJob) DeepCopyInto(out *MedusaBackupJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupJob.
func (in *MedusaBackupJob) DeepCopy() *MedusaBackupJob {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaBackupJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupJobList) DeepCopyInto(out *MedusaBackupJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MedusaBackupJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupJobList.
func (in *MedusaBackupJobList) DeepCopy() *MedusaBackupJobList {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupJobList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaBackupJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupJobSpec) DeepCopyInto(out *MedusaBackupJobSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupJobSpec.
func (in *MedusaBackupJobSpec) DeepCopy() *MedusaBackupJobSpec {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupJobSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupJobStatus) DeepCopyInto(out *MedusaBackupJobStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
	if in.InProgress != nil {
		in, out := &in.InProgress, &out.InProgress
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Finished != nil {
		in, out := &in.Finished, &out.Finished
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Failed != nil {
		in, out := &in.Failed, &out.Failed
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupJobStatus.
func (in *MedusaBackupJobStatus) DeepCopy() *MedusaBackupJobStatus {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupJobStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupList) DeepCopyInto(out *MedusaBackupList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MedusaBackup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupList.
func (in *MedusaBackupList) DeepCopy() *MedusaBackupList {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaBackupList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupSpec) DeepCopyInto(out *MedusaBackupSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupSpec.
func (in *MedusaBackupSpec) DeepCopy() *MedusaBackupSpec {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaBackupStatus) DeepCopyInto(out *MedusaBackupStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaBackupStatus.
func (in *MedusaBackupStatus) DeepCopy() *MedusaBackupStatus {
	if in == nil {
		return nil
	}
	out := new(MedusaBackupStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaClusterTemplate) DeepCopyInto(out *MedusaClusterTemplate) {
	*out = *in
	if in.ContainerImage != nil {
		in, out := &in.ContainerImage, &out.ContainerImage
		*out = new(images.Image)
		(*in).DeepCopyInto(*out)
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(v1.SecurityContext)
		(*in).DeepCopyInto(*out)
	}
	out.CassandraUserSecretRef = in.CassandraUserSecretRef
	in.StorageProperties.DeepCopyInto(&out.StorageProperties)
	out.CertificatesSecretRef = in.CertificatesSecretRef
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(v1.Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(v1.Probe)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaClusterTemplate.
func (in *MedusaClusterTemplate) DeepCopy() *MedusaClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(MedusaClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaRestoreJob) DeepCopyInto(out *MedusaRestoreJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaRestoreJob.
func (in *MedusaRestoreJob) DeepCopy() *MedusaRestoreJob {
	if in == nil {
		return nil
	}
	out := new(MedusaRestoreJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaRestoreJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaRestoreJobList) DeepCopyInto(out *MedusaRestoreJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MedusaRestoreJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaRestoreJobList.
func (in *MedusaRestoreJobList) DeepCopy() *MedusaRestoreJobList {
	if in == nil {
		return nil
	}
	out := new(MedusaRestoreJobList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaRestoreJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaRestoreJobSpec) DeepCopyInto(out *MedusaRestoreJobSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaRestoreJobSpec.
func (in *MedusaRestoreJobSpec) DeepCopy() *MedusaRestoreJobSpec {
	if in == nil {
		return nil
	}
	out := new(MedusaRestoreJobSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaRestoreJobStatus) DeepCopyInto(out *MedusaRestoreJobStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
	in.DatacenterStopped.DeepCopyInto(&out.DatacenterStopped)
	if in.InProgress != nil {
		in, out := &in.InProgress, &out.InProgress
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Finished != nil {
		in, out := &in.Finished, &out.Finished
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Failed != nil {
		in, out := &in.Failed, &out.Failed
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaRestoreJobStatus.
func (in *MedusaRestoreJobStatus) DeepCopy() *MedusaRestoreJobStatus {
	if in == nil {
		return nil
	}
	out := new(MedusaRestoreJobStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaTask) DeepCopyInto(out *MedusaTask) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaTask.
func (in *MedusaTask) DeepCopy() *MedusaTask {
	if in == nil {
		return nil
	}
	out := new(MedusaTask)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaTask) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaTaskList) DeepCopyInto(out *MedusaTaskList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MedusaTask, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaTaskList.
func (in *MedusaTaskList) DeepCopy() *MedusaTaskList {
	if in == nil {
		return nil
	}
	out := new(MedusaTaskList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MedusaTaskList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaTaskSpec) DeepCopyInto(out *MedusaTaskSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaTaskSpec.
func (in *MedusaTaskSpec) DeepCopy() *MedusaTaskSpec {
	if in == nil {
		return nil
	}
	out := new(MedusaTaskSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MedusaTaskStatus) DeepCopyInto(out *MedusaTaskStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
	if in.InProgress != nil {
		in, out := &in.InProgress, &out.InProgress
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Finished != nil {
		in, out := &in.Finished, &out.Finished
		*out = make([]TaskResult, len(*in))
		copy(*out, *in)
	}
	if in.Failed != nil {
		in, out := &in.Failed, &out.Failed
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MedusaTaskStatus.
func (in *MedusaTaskStatus) DeepCopy() *MedusaTaskStatus {
	if in == nil {
		return nil
	}
	out := new(MedusaTaskStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodStorageSettings) DeepCopyInto(out *PodStorageSettings) {
	*out = *in
	out.Size = in.Size.DeepCopy()
	if in.AccessModes != nil {
		in, out := &in.AccessModes, &out.AccessModes
		*out = make([]v1.PersistentVolumeAccessMode, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodStorageSettings.
func (in *PodStorageSettings) DeepCopy() *PodStorageSettings {
	if in == nil {
		return nil
	}
	out := new(PodStorageSettings)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
	out.StorageSecretRef = in.StorageSecretRef
	if in.PodStorage != nil {
		in, out := &in.PodStorage, &out.PodStorage
		*out = new(PodStorageSettings)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TaskResult) DeepCopyInto(out *TaskResult) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TaskResult.
func (in *TaskResult) DeepCopy() *TaskResult {
	if in == nil {
		return nil
	}
	out := new(TaskResult)
	in.DeepCopyInto(out)
	return out
}
