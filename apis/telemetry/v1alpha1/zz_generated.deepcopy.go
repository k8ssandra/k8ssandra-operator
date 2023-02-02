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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraAgentSpec) DeepCopyInto(out *CassandraAgentSpec) {
	*out = *in
	out.Endpoint = in.Endpoint
	if in.Filters != nil {
		in, out := &in.Filters, &out.Filters
		*out = make([]monitoringv1.RelabelConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraAgentSpec.
func (in *CassandraAgentSpec) DeepCopy() *CassandraAgentSpec {
	if in == nil {
		return nil
	}
	out := new(CassandraAgentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CassandraTelemetrySpec) DeepCopyInto(out *CassandraTelemetrySpec) {
	*out = *in
	if in.TelemetrySpec != nil {
		in, out := &in.TelemetrySpec, &out.TelemetrySpec
		*out = new(TelemetrySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Mcac != nil {
		in, out := &in.Mcac, &out.Mcac
		*out = new(McacTelemetrySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Cassandra != nil {
		in, out := &in.Cassandra, &out.Cassandra
		*out = new(CassandraAgentSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CassandraTelemetrySpec.
func (in *CassandraTelemetrySpec) DeepCopy() *CassandraTelemetrySpec {
	if in == nil {
		return nil
	}
	out := new(CassandraTelemetrySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Endpoint) DeepCopyInto(out *Endpoint) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Endpoint.
func (in *Endpoint) DeepCopy() *Endpoint {
	if in == nil {
		return nil
	}
	out := new(Endpoint)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *McacTelemetrySpec) DeepCopyInto(out *McacTelemetrySpec) {
	*out = *in
	if in.MetricFilters != nil {
		in, out := &in.MetricFilters, &out.MetricFilters
		*out = new([]string)
		if **in != nil {
			in, out := *in, *out
			*out = make([]string, len(*in))
			copy(*out, *in)
		}
	}
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new McacTelemetrySpec.
func (in *McacTelemetrySpec) DeepCopy() *McacTelemetrySpec {
	if in == nil {
		return nil
	}
	out := new(McacTelemetrySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PrometheusTelemetrySpec) DeepCopyInto(out *PrometheusTelemetrySpec) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	if in.CommonLabels != nil {
		in, out := &in.CommonLabels, &out.CommonLabels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PrometheusTelemetrySpec.
func (in *PrometheusTelemetrySpec) DeepCopy() *PrometheusTelemetrySpec {
	if in == nil {
		return nil
	}
	out := new(PrometheusTelemetrySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TelemetrySpec) DeepCopyInto(out *TelemetrySpec) {
	*out = *in
	if in.Prometheus != nil {
		in, out := &in.Prometheus, &out.Prometheus
		*out = new(PrometheusTelemetrySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Vector != nil {
		in, out := &in.Vector, &out.Vector
		*out = new(VectorSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TelemetrySpec.
func (in *TelemetrySpec) DeepCopy() *TelemetrySpec {
	if in == nil {
		return nil
	}
	out := new(TelemetrySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VectorComponentsSpec) DeepCopyInto(out *VectorComponentsSpec) {
	*out = *in
	if in.Sources != nil {
		in, out := &in.Sources, &out.Sources
		*out = make([]VectorSourceSpec, len(*in))
		copy(*out, *in)
	}
	if in.Sinks != nil {
		in, out := &in.Sinks, &out.Sinks
		*out = make([]VectorSinkSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Transforms != nil {
		in, out := &in.Transforms, &out.Transforms
		*out = make([]VectorTransformSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VectorComponentsSpec.
func (in *VectorComponentsSpec) DeepCopy() *VectorComponentsSpec {
	if in == nil {
		return nil
	}
	out := new(VectorComponentsSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VectorSinkSpec) DeepCopyInto(out *VectorSinkSpec) {
	*out = *in
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VectorSinkSpec.
func (in *VectorSinkSpec) DeepCopy() *VectorSinkSpec {
	if in == nil {
		return nil
	}
	out := new(VectorSinkSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VectorSourceSpec) DeepCopyInto(out *VectorSourceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VectorSourceSpec.
func (in *VectorSourceSpec) DeepCopy() *VectorSourceSpec {
	if in == nil {
		return nil
	}
	out := new(VectorSourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VectorSpec) DeepCopyInto(out *VectorSpec) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(v1.ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.ScrapeInterval != nil {
		in, out := &in.ScrapeInterval, &out.ScrapeInterval
		*out = new(metav1.Duration)
		**out = **in
	}
	if in.Components != nil {
		in, out := &in.Components, &out.Components
		*out = new(VectorComponentsSpec)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VectorSpec.
func (in *VectorSpec) DeepCopy() *VectorSpec {
	if in == nil {
		return nil
	}
	out := new(VectorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VectorTransformSpec) DeepCopyInto(out *VectorTransformSpec) {
	*out = *in
	if in.Inputs != nil {
		in, out := &in.Inputs, &out.Inputs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VectorTransformSpec.
func (in *VectorTransformSpec) DeepCopy() *VectorTransformSpec {
	if in == nil {
		return nil
	}
	out := new(VectorTransformSpec)
	in.DeepCopyInto(out)
	return out
}
