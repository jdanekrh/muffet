package muffet

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckerOptionsInitialize(t *testing.T) {
	o := checkerOptions{}
	o.Initialize()

	assert.Equal(t, defaultConcurrency, o.Concurrency)
}
