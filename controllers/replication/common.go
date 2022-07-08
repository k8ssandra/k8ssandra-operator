package replication

import (
	"context"
	"strings"

	coreapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Move to pkg when done
func objectRequiresUpdate(source, dest client.Object) bool {
	// In case we target the same cluster
	if source.GetUID() == dest.GetUID() {
		return false
	}

	if srcHash, found := source.GetAnnotations()[coreapi.ResourceHashAnnotation]; found {
		// Get dest hash value
		destHash, destFound := dest.GetAnnotations()[coreapi.ResourceHashAnnotation]
		if !destFound {
			return true
		}

		hash := getObjectHash(dest)
		if destHash != hash {
			// Destination data did not match destination hash
			return true
		}

		return srcHash != destHash
	}
	return false
}

func syncMetadata(src, dest *metav1.ObjectMeta) {
	// sync annotations, src is more important
	for k, v := range src.GetAnnotations() {
		if !datacenterSpecific(k) {
			metav1.SetMetaDataAnnotation(dest, k, v)
			dest.Annotations[k] = v
		}
	}

	// sync labels, src is more important
	for k, v := range src.Labels {
		if !datacenterSpecific(k) {
			metav1.SetMetaDataLabel(dest, k, v)
		}
	}
}

// filterValue verifies the annotation is not something datacenter specific
func datacenterSpecific(key string) bool {
	return strings.HasPrefix(key, "cassandra.datastax.com/")
}

func verifyHashAnnotation(ctx context.Context, localClient client.Client, sec client.Object) error {
	hash := getObjectHash(sec)
	if sec.GetAnnotations() == nil {
		sec.SetAnnotations(make(map[string]string))
	}
	if existingHash, found := sec.GetAnnotations()[coreapi.ResourceHashAnnotation]; !found || (existingHash != hash) {
		sec.GetAnnotations()[coreapi.ResourceHashAnnotation] = hash
		return localClient.Update(ctx, sec)
	}
	return nil
}

func getObjectHash(target client.Object) string {
	var obj interface{}
	switch v := target.(type) {
	case *corev1.ConfigMap:
		obj = v.Data
	case *corev1.Secret:
		obj = v.Data
	}
	hash := utils.DeepHashString(obj)
	return hash
}
