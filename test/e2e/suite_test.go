package e2e

import (
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"testing"
)

func TestOperator(t *testing.T) {
	beforeSuite(t)
}

func beforeSuite(t *testing.T) {
	framework.Init(t)
}
