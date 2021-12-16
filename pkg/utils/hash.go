package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"k8s.io/kubernetes/pkg/util/hash"
)

func DeepHashString(obj interface{}) string {
	hasher := sha256.New()
	hash.DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}
