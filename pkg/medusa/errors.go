package medusa

import "errors"

var (
	// This error indicates that a pod (or pods) do not include the medusa backup sidecar
	// container.
	ErrBackupSidecarNotFound = errors.New("the backup sidecar was not found")
)

// TerminalError marks a failure that a retry cannot fix.
// Errors that are not wrapped in a TerminalError are transient and are expected to be retried.
type TerminalError struct{ Err error }

// Error returns the text of the wrapped error.
func (e *TerminalError) Error() string { return e.Err.Error() }

// Unwrap returns the wrapped error, so that errors.Is and errors.As see through a TerminalError.
func (e *TerminalError) Unwrap() error { return e.Err }

// Terminal wraps err in a TerminalError. It returns nil if err is nil.
func Terminal(err error) error {
	if err == nil {
		return nil
	}
	return &TerminalError{Err: err}
}
