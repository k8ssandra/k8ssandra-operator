package utils

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestIsNil(t *testing.T) {
	t.Run("untyped nil", func(t *testing.T) {
		assert.True(t, IsNil(nil))
	})
	t.Run("struct", func(t *testing.T) {
		var v time.Location
		assert.False(t, IsNil(v))
	})
	t.Run("pointer", func(t *testing.T) {
		var v *time.Location
		assert.True(t, IsNil(v))
		v, _ = time.LoadLocation("Europe/Paris")
		assert.False(t, IsNil(v))
	})
	t.Run("map", func(t *testing.T) {
		var v map[string]string
		assert.True(t, IsNil(v))
		v = make(map[string]string, 0)
		assert.False(t, IsNil(v))
	})
	t.Run("slice", func(t *testing.T) {
		var v []string
		assert.True(t, IsNil(v))
		v = make([]string, 0)
		assert.False(t, IsNil(v))
	})
	t.Run("array", func(t *testing.T) {
		var v [1]string
		assert.False(t, IsNil(v))
	})
	t.Run("chan", func(t *testing.T) {
		var v chan string
		assert.True(t, IsNil(v))
		v = make(chan string)
		assert.False(t, IsNil(v))
	})
	t.Run("func", func(t *testing.T) {
		var v func() string
		assert.True(t, IsNil(v))
		v = func() string { return "foo" }
		assert.False(t, IsNil(v))
	})
	t.Run("interface", func(t *testing.T) {
		var v interface{}
		assert.True(t, IsNil(v))
		v = errors.New("foo")
		assert.False(t, IsNil(v))
	})
}
