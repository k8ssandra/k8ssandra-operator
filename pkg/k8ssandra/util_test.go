package k8ssandra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDcNameOverride(t *testing.T) {
	assert := assert.New(t)

	t.Run("with non-nil string pointer", func(t *testing.T) {
		datacenterName := "Test_Datacenter"
		got := dcNameOverride(&datacenterName)
		assert.Equal(datacenterName, got, "The two strings should be the same")
	})

	t.Run("with nil string pointer", func(t *testing.T) {
		got := dcNameOverride(nil)
		assert.Equal("", got, "Without a string pointer, the output should be an empty string")
	})

	t.Run("with empty string pointer", func(t *testing.T) {
		datacenterName := ""
		got := dcNameOverride(&datacenterName)
		assert.Equal("", got, "With an empty string pointer, the output should be an empty string")
	})

	t.Run("with string containing spaces pointer", func(t *testing.T) {
		datacenterName := "  "
		got := dcNameOverride(&datacenterName)
		assert.Equal(datacenterName, got, "The two strings (with spaces) should be the same")
	})
}
