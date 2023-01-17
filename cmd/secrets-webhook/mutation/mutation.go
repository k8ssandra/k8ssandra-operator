package mutation

import (
	"encoding/json"

	"github.com/wI2L/jsondiff"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// Mutator is a container for mutation
type Mutator struct {
	Logger *zap.Logger
}

// NewMutator returns an initialised instance of Mutator
func NewMutator(logger *zap.Logger) *Mutator {
	return &Mutator{Logger: logger}
}

// podMutators is an interface used to group functions mutating pods
type podMutator interface {
	Mutate(*corev1.Pod) (*corev1.Pod, error)
	Name() string
}

// MutatePodPatch returns a json patch containing all the mutations needed for
// a given pod
func (m *Mutator) MutatePodPatch(pod *corev1.Pod) ([]byte, error) {
	var podName string
	if pod.Name != "" {
		podName = pod.Name
	} else {
		if pod.ObjectMeta.GenerateName != "" {
			podName = pod.ObjectMeta.GenerateName
		}
	}
	log := m.Logger.With(zap.String("pod_name", podName))

	// list of all mutations to be applied to the pod
	mutations := []podMutator{
		injectSecrets{Logger: log},
	}

	mpod := pod.DeepCopy()

	// apply all mutations
	for _, m := range mutations {
		var err error
		mpod, err = m.Mutate(mpod)
		if err != nil {
			return nil, err
		}
	}

	// generate json patch
	patch, err := jsondiff.Compare(pod, mpod)
	if err != nil {
		return nil, err
	}

	patchb, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	return patchb, nil
}
