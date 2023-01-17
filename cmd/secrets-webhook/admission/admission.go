package admission

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/k8ssandra/k8ssandra-operator/cmd/secrets-webhook/mutation"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Admitter is a container for admission business
type Admitter struct {
	Logger  *zap.Logger
	Request *admissionv1.AdmissionRequest
}

// MutatePodReview takes an admission request and mutates the pod within,
// it returns an admission review with mutations as a json patch (if any)
func (a Admitter) MutatePodReview() (*admissionv1.AdmissionReview, error) {
	pod, err := a.Pod()
	if err != nil {
		e := fmt.Sprintf("could not parse pod in admission review request: %v", err)
		return reviewResponse(a.Request.UID, false, http.StatusBadRequest, e), err
	}

	m := mutation.NewMutator(a.Logger)
	patch, err := m.MutatePodPatch(pod)
	if err != nil {
		e := fmt.Sprintf("could not mutate pod: %v", err)
		return reviewResponse(a.Request.UID, false, http.StatusBadRequest, e), err
	}

	return patchReviewResponse(a.Request.UID, patch)
}

// Pod extracts a pod from an admission request
func (a Admitter) Pod() (*corev1.Pod, error) {
	if a.Request.Kind.Kind != "Pod" {
		return nil, fmt.Errorf("only pods are supported here")
	}

	p := corev1.Pod{}
	if err := json.Unmarshal(a.Request.Object.Raw, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// reviewResponse creates an admission review for a bad request
func reviewResponse(uid types.UID, allowed bool, httpCode int32,
	reason string) *admissionv1.AdmissionReview {
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: allowed,
			Result: &metav1.Status{
				Code:    httpCode,
				Message: reason,
			},
		},
	}
}

// patchReviewResponse builds an admission review with given json patch
func patchReviewResponse(uid types.UID, patch []byte) (*admissionv1.AdmissionReview, error) {
	patchType := admissionv1.PatchTypeJSONPatch

	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:       uid,
			Allowed:   true,
			PatchType: &patchType,
			Patch:     patch,
		},
	}, nil
}
