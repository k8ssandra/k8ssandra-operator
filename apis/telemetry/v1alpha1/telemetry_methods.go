package v1alpha1

import (
	"errors"

	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

// Merge takes an object a and merges another object, b's values into it, overwriting any which conflict.
func (a *TelemetrySpec) Merge(b *TelemetrySpec) *TelemetrySpec {
	// TODO: This method is brittle. It must be updated whenever any field is added to `TelemetrySpec``. It
	// would be best to replace these methods with a generic client side `Merge()` based on the k8s strategic merge
	// patch logic.
	out := TelemetrySpec{}
	if a == nil && b == nil {
		return &out
	} else if b == nil {
		return a
	} else if a == nil {
		return b
	}
	if b.Prometheus == nil {
		out.Prometheus = a.Prometheus
	} else {
		out.Prometheus = a.Prometheus.Merge(b.Prometheus)
	}
	return &out
}

// Merge takes an object a and merges another object, b's values into it, overwriting any which conflict.
func (a *PrometheusTelemetrySpec) Merge(b *PrometheusTelemetrySpec) *PrometheusTelemetrySpec {
	// TODO: This method is brittle. It must be updated whenever any field is added to `PrometheusTelemetrySpec`. It
	// would be best to replace these methods with a generic client side `Merge()` based on the k8s strategic merge
	// patch logic.
	out := PrometheusTelemetrySpec{}
	if a == nil && b == nil {
		return nil
	} else if b == nil {
		return a
	} else if a == nil {
		return b
	}
	if a.CommonLabels != nil {
		for k, v := range a.CommonLabels {
			out.CommonLabels[k] = v
		}
	}
	if b.CommonLabels != nil {
		for k, v := range b.CommonLabels {
			out.CommonLabels[k] = v
		}
	}
	if a.Enabled {
		out.Enabled = a.Enabled
	}
	return &out
}

func (tspec *TelemetrySpec) IsValid(client client.Client, logger logr.Logger) (bool, error) {
	promInstalled, err := IsPromInstalled(client, logger)
	if err != nil {
		return false, err
	}
	switch {
	case tspec == nil:
		return true, nil
	case tspec.Prometheus == nil:
		return true, nil
	case tspec.Prometheus.Enabled && !promInstalled:
		return false, nil
	case tspec.Prometheus.Enabled && promInstalled:
		return true, nil
	}
	return false, errors.New("something unexpected happened when determining if telemetry spec was valid")
}

// IsPromInstalled returns true if Prometheus is installed in the cluster, false otherwise.
func IsPromInstalled(client client.Client, logger logr.Logger) (bool, error) {
	promKinds, err := client.RESTMapper().KindsFor(promapi.SchemeGroupVersion.WithResource("servicemonitors"))
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		} else {
			logger.Error(err, "unable to tell if Prometheus installed", "errtype", reflect.TypeOf(err))
			return false, err
		}
	} else if promKinds != nil {
		return true, nil
	}
	return false, errors.New("something unexpected happened when determining whether prometheus is installed")
}
