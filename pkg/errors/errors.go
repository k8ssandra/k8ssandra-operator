package errors

import (
	"errors"
	"fmt"
)

type Reason string

const (
	// ReasonUnknown means that the underlying cause for the error is unknown
	ReasonUnknown = "Unknown"

	// ReasonSchemaDisagreement means that there is schema disagreement in the Cassandra
	// cluster so schema changes should not be applied until that disagreement is
	// resolved.
	ReasonSchemaDisagreement = "SchemaDisagreement"
)

type K8ssandraError struct {
	Reason  Reason
	Message string
}

func (e *K8ssandraError) Error() string {
	return fmt.Sprintf("%s: reason (%s)", e.Message, e.Reason)
}

func NewSchemaDisagreementError(message string) *K8ssandraError {
	return &K8ssandraError{
		Message: message,
		Reason:  ReasonSchemaDisagreement,
	}
}

func IsSchemaDisagreement(err error) bool {
	return ReasonForError(err) == ReasonSchemaDisagreement
}

func ReasonForError(err error) Reason {
	var k8ssandraError *K8ssandraError
	if errors.As(err, &k8ssandraError) {
		return k8ssandraError.Reason
	}
	return ReasonUnknown
}
