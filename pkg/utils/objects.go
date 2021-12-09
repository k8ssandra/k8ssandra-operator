package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetKey(obj metav1.Object) client.ObjectKey {
	return client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}
}
