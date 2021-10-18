package config

import (
	"log"
	"os"
	"time"
)

type ReconcilerConfig struct {
	DefaultDelay time.Duration
	LongDelay    time.Duration
}

const (
	RequeueDefaultDelayEnvVar = "REQUEUE_DEFAULT_DELAY"
	RequeueLongDelayEnvVar    = "REQUEUE_LONG_DELAY"
)

// InitConfig is primarily a hook for integration tests. It provides a way to use shorter
// requeue delays which allows the tests to run much faster. Note that this code will
// likely be changed when we tackle
// https://github.com/k8ssandra/k8ssandra-operator/issues/63.
func InitConfig() *ReconcilerConfig {
	var (
		defaultDelay time.Duration
		longDelay    time.Duration
		err          error
	)

	val, found := os.LookupEnv(RequeueDefaultDelayEnvVar)
	if found {
		defaultDelay, err = time.ParseDuration(val)
		if err != nil {
			log.Fatalf("failed to parse value for %s %s: %s", RequeueDefaultDelayEnvVar, val, err)
		}
	} else {
		defaultDelay = 15 * time.Second
	}

	val, found = os.LookupEnv(RequeueLongDelayEnvVar)
	if found {
		longDelay, err = time.ParseDuration(val)
		if err != nil {
			log.Fatalf("failed to parse value for %s %s: %s", RequeueLongDelayEnvVar, val, err)
		}
	} else {
		longDelay = 1 * time.Minute
	}

	return &ReconcilerConfig{
		DefaultDelay: defaultDelay,
		LongDelay:    longDelay,
	}
}
