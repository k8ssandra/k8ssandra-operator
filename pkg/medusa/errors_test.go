package medusa

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTerminalError(t *testing.T) {
	inner := errors.New("inner failure")
	err := Terminal(inner)

	assert.Equal(t, "inner failure", err.Error())
	assert.True(t, errors.Is(err, inner))

	var terminal *TerminalError
	assert.True(t, errors.As(err, &terminal))
	assert.Same(t, inner, terminal.Err)
}

func TestTerminalErrorWrappedAgain(t *testing.T) {
	inner := errors.New("inner failure")
	err := fmt.Errorf("outer: %w", Terminal(inner))

	var terminal *TerminalError
	assert.True(t, errors.As(err, &terminal))
	assert.True(t, errors.Is(err, inner))
	assert.Equal(t, "outer: inner failure", err.Error())
}

func TestTerminalNil(t *testing.T) {
	assert.NoError(t, Terminal(nil))
}

func TestPlainErrorIsNotTerminal(t *testing.T) {
	var terminal *TerminalError
	assert.False(t, errors.As(fmt.Errorf("plain: %w", errors.New("x")), &terminal))
}
