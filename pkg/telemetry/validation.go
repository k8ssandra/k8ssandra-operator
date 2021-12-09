package telemetry

import (
	"errors"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	promapi "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

// IsPromInstalled returns true if Prometheus is installed in the cluster, false otherwise.
func IsPromInstalled(client client.Client, logger logr.Logger) (bool, error) {
	promKinds, err := client.RESTMapper().KindsFor(promapi.SchemeGroupVersion.WithResource("servicemonitors"))
	if err != nil {
		if meta.IsNoMatchError(err) {
			logger.Info("Prometheus does not appear to be installed, proceeding")
			return false, nil
		} else {
			logger.Error(err, "unable to tell if Prometheus installed", "errtype", reflect.TypeOf(err))
			return false, err
		}
	} else if promKinds != nil {
		logger.Info("Prometheus appears to be installed, adding to scheme", "promKinds", promKinds)
		return true, nil
	}
	return false, errors.New("something unexpected happened when determining whether prometheus is installed")
}
