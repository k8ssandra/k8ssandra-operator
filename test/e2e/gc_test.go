package e2e

import (
	"context"
	api "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func gcTest(gcName string) e2eTestFunc {
	return func(t *testing.T, ctx context.Context, namespace string, f *framework.E2eFramework) {

		t.Log("check that the K8ssandraCluster was created")
		kc := &api.K8ssandraCluster{}
		kcKey := types.NamespacedName{Namespace: namespace, Name: "test"}
		err := f.Client.Get(ctx, kcKey, kc)
		require.NoError(t, err)

		dc1Key := framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1")
		checkDatacenterReady(t, ctx, dc1Key, f)

		logs, err := f.GetContainerLogs(f.DataPlaneContexts[0], namespace, "test-dc1-default-sts-0", "server-system-logger")
		assert.Contains(t, logs, gcName)

		switch gcName {
		case "G1":
			checkLogsForG1(t, logs)
		case "CMS":
			checkLogsForCMS(t, logs)
		case "ZGC":
			checkLogsForZGC(t, logs)
		}
	}
}

func checkLogsForG1(t *testing.T, logs string) {
	assert.Contains(t, logs, "G1 Eden Space Heap memory")
	assert.Contains(t, logs, "G1 Survivor Space Heap memory")
	assert.Contains(t, logs, "G1 Old Gen Heap memory")
	assert.Contains(t, logs, "-XX:+UseG1GC")
	assert.Contains(t, logs, "-XX:ConcGCThreads=2")
	assert.Contains(t, logs, "-XX:ParallelGCThreads=2")
	assert.Contains(t, logs, "-XX:G1RSetUpdatingPauseTimePercent=6")
	assert.Contains(t, logs, "-XX:InitiatingHeapOccupancyPercent=75")
	assert.Contains(t, logs, "-XX:MaxGCPauseMillis=600")
}

func checkLogsForCMS(t *testing.T, logs string) {
	assert.Contains(t, logs, "CMS Old Gen Heap memory")
	// automatically set by config-builder
	assert.Contains(t, logs, "-XX:+UseConcMarkSweepGC")
	assert.Contains(t, logs, "-XX:+CMSParallelRemarkEnabled")
	assert.Contains(t, logs, "-XX:+UseCMSInitiatingOccupancyOnly")
	assert.Contains(t, logs, "-XX:+CMSParallelInitialMarkEnabled")
	assert.Contains(t, logs, "-XX:+CMSEdenChunksRecordAlways")
	assert.Contains(t, logs, "-XX:+CMSClassUnloadingEnabled")
	// user settings
	assert.Contains(t, logs, "-XX:SurvivorRatio=4")
	assert.Contains(t, logs, "-XX:MaxTenuringThreshold=2")
	assert.Contains(t, logs, "-XX:CMSInitiatingOccupancyFraction=76")
	assert.Contains(t, logs, "-XX:CMSWaitDuration=11000")
}

func checkLogsForZGC(t *testing.T, logs string) {
	assert.Contains(t, logs, "ZHeap Heap memory")
	assert.Contains(t, logs, "-XX:+UseZGC")
	assert.Contains(t, logs, "-XX:ConcGCThreads=1")
}
