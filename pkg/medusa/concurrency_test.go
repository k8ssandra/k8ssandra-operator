package medusa

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTaskConcurrency(t *testing.T) {
	for value, expected := range map[string]TaskConcurrency{"": TaskConcurrencyAll, "all": TaskConcurrencyAll, "rack": TaskConcurrencyRack, "pod": TaskConcurrencyPod} {
		concurrency, ok := ParseTaskConcurrency(value)
		require.True(t, ok, value)
		require.Equal(t, expected, concurrency, value)
	}

	concurrency, ok := ParseTaskConcurrency("racks")
	require.False(t, ok)
	require.Equal(t, TaskConcurrencyPod, concurrency)
}
