package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"hash"

	"github.com/davecgh/go-spew/spew"
)

const (
	// The system wide length of the hash to use for object identification.
	hashLength = 8
)

func DeepHashString(obj interface{}) string {
	hasher := sha256.New()
	DeepHashObject(hasher, obj)
	hashBytes := hasher.Sum([]byte{})
	b64Hash := base64.StdEncoding.EncodeToString(hashBytes)
	return b64Hash
}

// DeepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", objectToWrite)
}

func HashNameNamespace(name, namespace string) string {
	h := sha256.New()
	h.Write([]byte(namespace + name))
	return string(h.Sum(nil))[:hashLength]
}
