package telemetry

import (
	"errors"
	"reflect"

	"github.com/go-logr/logr"
	telemetryapi "github.com/k8ssandra/k8ssandra-operator/apis/telemetry/v1alpha1"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SpecIsValid(tspec *telemetryapi.TelemetrySpec, promInstalled bool) bool {
	switch {
	case tspec == nil:
		return true
	case tspec.Prometheus == nil:
		return true
	case tspec.IsPrometheusEnabled():
		return promInstalled
	default:
		return true
	}
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
