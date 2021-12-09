// This file holds functions and types relating to prometheus telemetry for Cassandra Datacenters.

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCassTelemetryResourcer_IS_TelemetryResourcer tests that CassTelemetryResourcer implements the Resourcer interface.
func TestCassTelemetryResourcer_IS_TelemetryResourcer(t *testing.T) {
	assert.Implements(t, (*Resourcer)(nil), CassTelemetryResourcer{})
}
