package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	k8ssandraapi "github.com/k8ssandra/k8ssandra-operator/apis/k8ssandra/v1alpha1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	medusapkg "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	pitrIntervalSeconds = 10
	pitrInterval        = time.Duration(pitrIntervalSeconds) * time.Second
	pitrGrace           = 120*time.Second + 2*pitrInterval
	pitrNodeCount       = 2
	pitrChurnBatches    = 16
	pitrEntryPointGap   = "https://github.com/thelastpickle/cassandra-medusa/issues/1029"
	pitrPrepareGap      = "https://github.com/thelastpickle/cassandra-medusa/issues/1027"
)

var errNoPitrWorkloadSegments = errors.New("no workload segments were archived on any pod")

type medusaPitrTest struct {
	ctx             context.Context
	f               *framework.E2eFramework
	cluster         framework.ClusterKey
	source          framework.ClusterKey
	prefix          string
	bucket          string
	toolbox         framework.ClusterKey
	firstBackup     string
	baseBackup      string
	betweenBackups  time.Time
	target          time.Time
	targetObjects   map[string]pitrObject
	beforeWorkload  map[string]pitrObject
	inPlaceStatus   medusa.MedusaRestoreJobStatus
	expectedFailure int
}

func newMedusaPitrTest(ctx context.Context, namespace string, f *framework.E2eFramework) *medusaPitrTest {
	return &medusaPitrTest{
		ctx:     ctx,
		f:       f,
		cluster: framework.NewClusterKey(f.ControlPlaneContext, namespace, clusterName),
		source:  framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "dc1"),
		toolbox: framework.NewClusterKey(f.DataPlaneContexts[0], namespace, "pitr-storage-client"),
	}
}

func (s *medusaPitrTest) enable(t *testing.T, prepareStorageOutage bool) {
	checkDatacenterReady(t, s.ctx, s.source, s.f)
	kc := s.getCluster(t)
	require.Nil(t, kc.Spec.Medusa.Pitr)
	s.prefix = kc.Spec.Medusa.StorageProperties.Prefix
	s.bucket = kc.Spec.Medusa.StorageProperties.BucketName
	if prepareStorageOutage {
		s.preparePitrStorage(t)
	}
	s.startStorageClient(t)
	pods := s.readyPods(t, s.source)
	s.checkLifecycle(t, pods, false)
	require.NoError(t, s.cql(s.ctx, pods[0].Name, `
CREATE KEYSPACE pitr_e2e WITH replication = {'class': 'NetworkTopologyStrategy', 'dc1': 2};
CREATE TABLE pitr_e2e.rows (id int PRIMARY KEY, value text);
CREATE TABLE pitr_e2e.churn (bucket int, id int, payload text, PRIMARY KEY (bucket, id));
CONSISTENCY ALL;
INSERT INTO pitr_e2e.rows (id, value) VALUES (1, 'base-one');
INSERT INTO pitr_e2e.rows (id, value) VALUES (2, 'base-two');
`))
	s.firstBackup = "pitr-before-enabling"
	s.backup(t, s.firstBackup)
	s.betweenBackups = s.podTime(t, pods[0].Name)

	s.setPitr(t, true)
	s.waitForRoll(t, pods, true)
}

func (s *medusaPitrTest) backupWithPitr(t *testing.T) {
	ini := s.medusaIni(t)
	require.Contains(t, ini, "[pitr]")
	require.Contains(t, ini, fmt.Sprintf("commitlog_archive_interval_seconds = %d", pitrIntervalSeconds))
	s.baseBackup = "pitr-base"
	// The rolling restart separates the two snapshots and the intervening timestamp.
	require.Greater(t, s.podTime(t, s.readyPods(t, s.source)[0].Name).Sub(s.betweenBackups), time.Second)
	s.backup(t, s.baseBackup)
}

func (s *medusaPitrTest) archive(t *testing.T) {
	pods := s.readyPods(t, s.source)
	s.checkLifecycle(t, pods, true)
	var err error
	s.beforeWorkload, err = s.objects()
	require.NoError(t, err)
	require.NoError(t, s.cql(s.ctx, pods[0].Name, `
CONSISTENCY ALL;
INSERT INTO pitr_e2e.rows (id, value) VALUES (3, 'replayed-and-later-deleted');
INSERT INTO pitr_e2e.rows (id, value) VALUES (4, 'replayed');
`))
	require.NoError(t, s.writeChurn(s.ctx, pods[0].Name, "ALL", pitrChurnBatches))
	// The legacy init container does not arm Cassandra archiving. Both image
	// paths validate the same per-node uploads and spool ages.
	legacyStartup := true
	for _, pod := range pods {
		log, err := s.logs(s.ctx, s.source, pod.Name, "medusa-restore", false)
		require.NoError(t, err)
		legacyStartup = legacyStartup && strings.Contains(log, "BACKUP_NAME env var not set, skipping restore operation")
	}
	if legacyStartup {
		err := s.waitForWorkloadArchive(pods, 3*pitrInterval)
		if err != nil && !errors.Is(err, errNoPitrWorkloadSegments) {
			require.NoError(t, err)
		}
		s.expectFailUntilMedusa(t, 3, pitrEntryPointGap, errNoPitrWorkloadSegments.Error(), func() error {
			return err
		})
		return
	}
	require.NoError(t, s.waitForWorkloadArchive(pods, polling.medusaPitrArchive.timeout))
}

func (s *medusaPitrTest) waitForWorkloadArchive(pods []corev1.Pod, timeout time.Duration) error {
	var lastCheck error
	err := wait.PollUntilContextTimeout(s.ctx, polling.medusaPitrArchive.interval, timeout, true, func(context.Context) (bool, error) {
		lastCheck = s.checkWorkloadArchive(pods)
		return lastCheck == nil, nil
	})
	if errors.Is(err, context.DeadlineExceeded) && s.ctx.Err() == nil && lastCheck != nil {
		return fmt.Errorf("timed out waiting for commitlog archiving: %w", lastCheck)
	}
	return err
}

func (s *medusaPitrTest) checkWorkloadArchive(pods []corev1.Pod) error {
	objects, err := s.objects()
	if err != nil {
		return err
	}
	anyUploads := false
	var shortfalls []string
	for _, pod := range pods {
		archived := objectsForPod(objects, pod.Name)
		maps.DeleteFunc(archived, func(_ string, object pitrObject) bool {
			_, existed := s.beforeWorkload[object.Key]
			return existed
		})
		anyUploads = anyUploads || len(archived) > 0
		if len(archived) < 2 {
			shortfalls = append(shortfalls, fmt.Sprintf("%s has %d", pod.Name, len(archived)))
		}
		spool, err := s.spool(pod.Name)
		if err != nil {
			return err
		}
		for name, age := range spool {
			if age > 2*pitrInterval {
				return fmt.Errorf("%s spool file %s is %s old, exceeding %s", pod.Name, name, age, 2*pitrInterval)
			}
		}
	}
	// The known legacy failure is total absence of uploads. Partial uploads,
	// stale spool files, and read errors must still fail the test.
	if !anyUploads {
		return errNoPitrWorkloadSegments
	}
	if len(shortfalls) > 0 {
		return fmt.Errorf("expected at least two workload segments per pod: %s", strings.Join(shortfalls, ", "))
	}
	return nil
}

func (s *medusaPitrTest) recordTarget(t *testing.T) {
	pods := s.readyPods(t, s.source)
	require.NoError(t, s.writeChurn(s.ctx, pods[0].Name, "ALL", pitrChurnBatches))
	var err error
	s.targetObjects, err = s.objects()
	require.NoError(t, err)
	maps.DeleteFunc(s.targetObjects, func(key string, _ pitrObject) bool {
		_, existed := s.beforeWorkload[key]
		return existed
	})
	s.target = s.podTime(t, pods[0].Name)
	t.Logf("PITR target: %s", s.target.Format(time.RFC3339Nano))
}

func (s *medusaPitrTest) writeAfterTarget(t *testing.T) {
	pods := s.readyPods(t, s.source)
	require.Eventually(t, func() bool {
		now, err := s.readPodTime(pods[0].Name)
		return err == nil && now.Sub(s.target) >= 2*time.Second
	}, pitrInterval, polling.medusaPitrArchive.interval)
	// A delete by partition key writes a partition tombstone. Cassandra 4.1.8
	// considers its timestamp in PartitionUpdate.maxTimestamp during replay.
	require.NoError(t, s.cql(s.ctx, pods[0].Name, `
CONSISTENCY ALL;
INSERT INTO pitr_e2e.rows (id, value) VALUES (5, 'after-target-one');
INSERT INTO pitr_e2e.rows (id, value) VALUES (6, 'after-target-two');
DELETE FROM pitr_e2e.rows WHERE id = 3;
`))
	s.verifyRows(t, s.source, clusterName, map[int]string{
		1: "base-one", 2: "base-two", 4: "replayed", 5: "after-target-one", 6: "after-target-two",
	})
}

func (s *medusaPitrTest) verifyInPlace(t *testing.T) {
	s.verifyRows(t, s.source, clusterName, pitrTargetRows())
}

func (s *medusaPitrTest) restoreOutOfPlace(t *testing.T) {
	kc := s.getCluster(t)
	const targetName = "pitr-target"
	targetContext := s.source.K8sContext
	if len(s.f.DataPlaneContexts) > 1 {
		// Resolve the fixture's context alias the same way as DeployFixture.
		contextAlias := kc.Annotations["e2e.k8ssandra.io/pitr-target-context"]
		index, err := strconv.Atoi(strings.TrimPrefix(contextAlias, "kind-k8ssandra-"))
		require.NoError(t, err)
		require.Positive(t, index)
		require.Less(t, index, len(s.f.DataPlaneContexts))
		targetContext = s.f.DataPlaneContexts[index]
		host, port := s.exposeStorageToOtherPlane(t)
		kc.Spec.Medusa.StorageProperties.Host = host
		kc.Spec.Medusa.StorageProperties.Port = port
	}

	kc.ObjectMeta = metav1.ObjectMeta{Namespace: s.cluster.Namespace, Name: targetName}
	kc.Status = k8ssandraapi.K8ssandraClusterStatus{}
	kc.Spec.Cassandra.ClusterName = targetName
	kc.Spec.Medusa.StorageProperties.Prefix = s.prefix
	// An out-of-place target shares the source backups and must replay with
	// PITR unset so the test also verifies that it does not archive.
	kc.Spec.Medusa.Pitr = nil
	dc := &kc.Spec.Cassandra.Datacenters[0]
	dc.Meta.Name = targetName + "-dc1"
	dc.DatacenterName = s.source.Name
	dc.K8sContext = targetContext
	target := framework.NewClusterKey(targetContext, s.source.Namespace, dc.Meta.Name)
	require.NoError(t, s.f.Client.Create(s.ctx, kc))
	t.Cleanup(func() { s.deleteObject(t, s.cluster, kc) })

	checkDatacenterReady(t, s.ctx, target, s.f)
	s.checkLifecycle(t, s.readyPods(t, target), false)
	job := s.startRestore(t, target, "pitr-out-of-place", s.target, "")
	job, err := s.waitForRestore(target, job, func(job *medusa.MedusaRestoreJob) bool {
		return job.Status.RestorePrepared || !job.Status.FinishTime.IsZero()
	})
	require.NoError(t, err)
	require.True(t, job.Status.RestorePrepared, job.Status.Message)

	var legacyRejection string
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		current := &cassdcapi.CassandraDatacenter{}
		require.NoError(c, s.f.Get(s.ctx, target, current))
		var restoreMapping medusa.MedusaRestoreMapping
		var rawMapping string
		for _, container := range current.Spec.PodTemplateSpec.Spec.InitContainers {
			if container.Name == "medusa-restore" {
				for _, env := range container.Env {
					if env.Name == "RESTORE_MAPPING" {
						rawMapping = env.Value
					}
				}
			}
		}
		require.NotEmpty(c, rawMapping)
		require.NoError(c, json.Unmarshal([]byte(rawMapping), &restoreMapping))
		require.Equal(c, ptr.To(false), restoreMapping.InPlace)
		require.NotNil(c, restoreMapping.Pitr)
		require.Equal(c, s.target.Format(time.RFC3339Nano), restoreMapping.Pitr.TargetTimestamp)
		require.Len(c, restoreMapping.Pitr.NodeBackups, pitrNodeCount)
		for _, base := range restoreMapping.Pitr.NodeBackups {
			require.Equal(c, s.baseBackup, base)
		}
		pods, err := s.pods(target)
		require.NoError(c, err)
		for _, pod := range pods {
			log, err := s.logs(s.ctx, target, pod.Name, "medusa-restore", false)
			if err == nil && strings.Contains(log, "PITR restore requested, but PITR is disabled") {
				legacyRejection = log
				return
			}
		}
		latest := &medusa.MedusaRestoreJob{}
		require.NoError(c, s.f.Get(s.ctx, pitrJobKey(target, job.Name), latest))
		require.False(c, latest.Status.FinishTime.IsZero())
		require.Empty(c, latest.Status.Message)
	}, polling.medusaRestoreDone.timeout, polling.medusaPitrArchive.interval)
	if legacyRejection != "" {
		s.expectFailUntilMedusa(t, 8, pitrEntryPointGap, "PITR restore requested, but PITR is disabled", func() error {
			return fmt.Errorf("restore init container rejected the request: %s", legacyRejection)
		})
		return
	}
	s.checkLifecycle(t, s.readyPods(t, target), false)
	s.verifyRows(t, target, targetName, pitrTargetRows())
}

func (s *medusaPitrTest) rejectBeforeBase(t *testing.T) {
	oldPods := s.readyPods(t, s.source)
	job := s.startRestore(t, s.source, "pitr-no-base", s.betweenBackups, "")
	job, err := s.waitForRestore(s.source, job, func(job *medusa.MedusaRestoreJob) bool {
		return job.Status.RestorePrepared || !job.Status.FinishTime.IsZero()
	})
	require.NoError(t, err)
	require.False(t, job.Status.RestorePrepared)
	require.True(t, job.Status.DatacenterStopped.IsZero())
	dc := &cassdcapi.CassandraDatacenter{}
	require.NoError(t, s.f.Get(s.ctx, s.source, dc))
	require.False(t, dc.Spec.Stopped)
	require.Equal(t, podUIDs(oldPods), podUIDs(s.readyPods(t, s.source)))
	require.False(t, job.Status.FinishTime.IsZero())
	const unsupported = "the Medusa image does not support PITR"
	if strings.Contains(job.Status.Message, unsupported) {
		s.expectFailUntilMedusa(t, 9, pitrPrepareGap, unsupported, func() error {
			return errors.New(job.Status.Message)
		})
		return
	}
	require.Contains(t, job.Status.Message, "rejected by Medusa")
}

func (s *medusaPitrTest) disable(t *testing.T) {
	pods := s.readyPods(t, s.source)
	watch := s.watchPods(t, s.source)
	s.setPitr(t, false)
	s.waitForRoll(t, pods, false)
	s.checkTermination(t, watch, pods, false)
	s.requireNoFailedHooks(t, pods)
	require.NotContains(t, s.medusaIni(t), "[pitr]")
}

func (s *medusaPitrTest) getCluster(t *testing.T) *k8ssandraapi.K8ssandraCluster {
	t.Helper()
	kc := &k8ssandraapi.K8ssandraCluster{}
	require.NoError(t, s.f.Get(s.ctx, s.cluster, kc))
	return kc
}

func (s *medusaPitrTest) medusaIni(t *testing.T) string {
	t.Helper()
	configMap := medusapkg.CreateMedusaConfigMap(s.source.Namespace, s.cluster.Name, "")
	key := framework.NewClusterKey(s.source.K8sContext, configMap.Namespace, configMap.Name)
	require.NoError(t, s.f.Get(s.ctx, key, configMap))
	require.NotEmpty(t, configMap.Data["medusa.ini"])
	return configMap.Data["medusa.ini"]
}

func (s *medusaPitrTest) setPitr(t *testing.T, enabled bool) {
	t.Helper()
	kc := s.getCluster(t)
	patch := client.MergeFromWithOptions(kc.DeepCopy(), client.MergeFromWithOptimisticLock{})
	kc.Spec.Medusa.Pitr = &medusa.PitrConfig{Enabled: enabled, ArchiveIntervalSeconds: ptr.To(pitrIntervalSeconds)}
	require.NoError(t, s.f.Patch(s.ctx, kc, patch, s.cluster))
}

func (s *medusaPitrTest) backup(t *testing.T, name string) {
	t.Helper()
	job := &medusa.MedusaBackupJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: s.source.Namespace, Name: name},
		Spec:       medusa.MedusaBackupJobSpec{CassandraDatacenter: s.source.Name},
	}
	key := pitrJobKey(s.source, name)
	require.NoError(t, s.f.Create(s.ctx, key, job))
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.f.Get(s.ctx, key, job))
		require.False(c, job.Status.FinishTime.IsZero())
		backup := &medusa.MedusaBackup{}
		require.NoError(c, s.f.Get(s.ctx, key, backup))
		require.Equal(c, medusapkg.StatusType_SUCCESS.String(), backup.Status.Status)
		require.Len(c, backup.Status.Nodes, pitrNodeCount)
		require.Equal(c, int32(pitrNodeCount), backup.Status.FinishedNodes)
	}, polling.medusaBackupDone.timeout, polling.medusaBackupDone.interval, "backup %s did not finish", name)
}

func pitrJobKey(dc framework.ClusterKey, name string) framework.ClusterKey {
	return framework.NewClusterKey(dc.K8sContext, dc.Namespace, name)
}

func (s *medusaPitrTest) startRestore(t *testing.T, dc framework.ClusterKey, name string, target time.Time, backup string) *medusa.MedusaRestoreJob {
	t.Helper()
	job := &medusa.MedusaRestoreJob{
		ObjectMeta: metav1.ObjectMeta{Namespace: dc.Namespace, Name: name},
		Spec:       medusa.MedusaRestoreJobSpec{CassandraDatacenter: dc.Name, Backup: backup},
	}
	if !target.IsZero() {
		job.Spec.TargetTimestamp = target.Format(time.RFC3339Nano)
	}
	key := pitrJobKey(dc, name)
	require.NoError(t, s.f.Create(s.ctx, key, job))
	t.Cleanup(func() { s.deleteObject(t, key, job) })
	return job
}

func (s *medusaPitrTest) deleteObject(t *testing.T, key framework.ClusterKey, obj client.Object) {
	t.Helper()
	require.NoError(t, client.IgnoreNotFound(s.f.Delete(s.ctx, key, obj)))
}

func (s *medusaPitrTest) waitForRestore(dc framework.ClusterKey, job *medusa.MedusaRestoreJob, done func(*medusa.MedusaRestoreJob) bool) (*medusa.MedusaRestoreJob, error) {
	key := pitrJobKey(dc, job.Name)
	latest := job.DeepCopy()
	err := wait.PollUntilContextTimeout(s.ctx, polling.medusaPitrArchive.interval, polling.medusaRestoreDone.timeout, true,
		func(ctx context.Context) (bool, error) {
			err := s.f.Get(ctx, key, latest)
			return err == nil && done(latest), err
		})
	if err != nil {
		return latest, fmt.Errorf("waiting for %s (status %+v): %w", key, latest.Status, err)
	}
	return latest, nil
}

func (s *medusaPitrTest) requireRestoreFinished(t *testing.T, dc framework.ClusterKey, job *medusa.MedusaRestoreJob) *medusa.MedusaRestoreJob {
	t.Helper()
	finished, err := s.waitForRestore(dc, job, func(job *medusa.MedusaRestoreJob) bool {
		return !job.Status.FinishTime.IsZero()
	})
	require.NoError(t, err)
	require.True(t, finished.Status.RestorePrepared, finished.Status.Message)
	require.Empty(t, finished.Status.Message)
	return finished
}

// expectFailUntilMedusa requires the known Medusa error. An unexpected
// success or unrelated error fails the sub-test.
func (s *medusaPitrTest) expectFailUntilMedusa(t *testing.T, step int, issue, failure string, check func() error) {
	t.Helper()
	s.expectedFailure = step
	t.Run("expected_failure", func(t *testing.T) {
		require.ErrorContains(t, check(), failure, "expected Medusa gap %s", issue)
		t.Logf("EXPECTED FAILURE: %s (%s)", failure, issue)
	})
}

func pitrTargetRows() map[int]string {
	return map[int]string{1: "base-one", 2: "base-two", 3: "replayed-and-later-deleted", 4: "replayed"}
}

func (s *medusaPitrTest) verifyRows(t *testing.T, dc framework.ClusterKey, cluster string, expected map[int]string) {
	t.Helper()
	for _, pod := range s.readyPods(t, dc) {
		var out string
		err := wait.PollUntilContextTimeout(s.ctx, polling.medusaReady.interval, polling.medusaReady.timeout, true,
			func(ctx context.Context) (bool, error) {
				var err error
				out, err = s.query(ctx, dc, cluster, pod.Name, "CONSISTENCY ALL;\nSELECT id, value FROM pitr_e2e.rows;")
				if errors.Is(err, context.DeadlineExceeded) || (err != nil && strings.Contains(err.Error(), "ReadTimeout:")) {
					t.Logf("Retrying timed-out row check on %s", pod.Name)
					return false, nil
				}
				return err == nil, err
			})
		require.NoError(t, err)
		rows := make(map[int]string)
		for line := range strings.SplitSeq(out, "\n") {
			id, value, found := strings.Cut(line, "|")
			if !found {
				continue
			}
			row, err := strconv.Atoi(strings.TrimSpace(id))
			if err == nil {
				rows[row] = strings.TrimSpace(value)
			}
		}
		require.Len(t, rows, len(expected), "%s returned %s", pod.Name, out)
		require.Equal(t, expected, rows, "rows on %s", pod.Name)
	}
}

func (s *medusaPitrTest) writeChurn(ctx context.Context, pod, consistency string, batches int) error {
	var workload strings.Builder
	fmt.Fprintf(&workload, "CONSISTENCY %s;\n", consistency)
	value := strings.Repeat("x", 16*1024)
	// Batch within one partition to reduce network round trips. Each
	// mutation is about 256 KiB, below half of a 1 MiB commitlog segment.
	for batch := range batches {
		workload.WriteString("BEGIN UNLOGGED BATCH\n")
		for row := range 16 {
			fmt.Fprintf(&workload, "INSERT INTO pitr_e2e.churn (bucket, id, payload) VALUES (0, %d, '%s');\n", batch*16+row, value)
		}
		workload.WriteString("APPLY BATCH;\n")
	}
	return s.cql(ctx, pod, workload.String())
}

func (s *medusaPitrTest) cql(ctx context.Context, pod, query string) error {
	_, err := s.query(ctx, s.source, clusterName, pod, query)
	return err
}

func (s *medusaPitrTest) query(ctx context.Context, dc framework.ClusterKey, cluster, pod, query string) (string, error) {
	username, password, err := s.f.RetrieveDatabaseCredentials(ctx, dc.K8sContext, dc.Namespace, cluster)
	if err != nil {
		return "", err
	}
	podKey := framework.NewClusterKey(dc.K8sContext, dc.Namespace, pod)
	// The largest workload exceeds the shell argument limit, and cqlsh -f
	// seeks its input. Stream it into a temporary regular file in the pod.
	return pitrKubectl(ctx, podKey, query, "exec", "-i", pod, "-c", "cassandra", "--",
		"/bin/sh", "-ec", `
file=$(mktemp)
trap 'rm -f "$file"' EXIT
cat > "$file"
cqlsh --no-color --username "$1" --password="$2" -f "$file"
`, "sh", username, password)
}

func (s *medusaPitrTest) readPodTime(pod string) (time.Time, error) {
	out, err := s.exec(s.ctx, s.source, pod, "cassandra", "date", "+%s%3N")
	if err != nil {
		return time.Time{}, err
	}
	millis, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return time.UnixMilli(millis).UTC(), err
}

func (s *medusaPitrTest) podTime(t *testing.T, pod string) time.Time {
	t.Helper()
	now, err := s.readPodTime(pod)
	require.NoError(t, err)
	return now
}

func podUIDs(pods []corev1.Pod) map[string]string {
	uids := make(map[string]string, len(pods))
	for _, pod := range pods {
		uids[pod.Name] = string(pod.UID)
	}
	return uids
}

func (s *medusaPitrTest) waitForRoll(t *testing.T, previous []corev1.Pod, enabled bool) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pods, err := s.pods(s.source)
		require.NoError(c, err)
		before, after := podUIDs(previous), podUIDs(pods)
		require.ElementsMatch(c, slices.Collect(maps.Keys(before)), slices.Collect(maps.Keys(after)))
		for _, pod := range pods {
			require.NotEqual(c, before[pod.Name], after[pod.Name], "%s did not roll", pod.Name)
			require.True(c, pitrPodReady(pod), "%s is not ready", pod.Name)
		}
		s.checkLifecycle(c, pods, enabled)
	}, polling.datacenterReady.timeout, polling.medusaPitrArchive.interval)
	checkDatacenterReady(t, s.ctx, s.source, s.f)
}

func (s *medusaPitrTest) checkLifecycle(t require.TestingT, pods []corev1.Pod, enabled bool) {
	grace := 120 * time.Second
	if enabled {
		grace = pitrGrace
	}
	for _, pod := range pods {
		require.Equal(t, ptr.To(int64(grace/time.Second)), pod.Spec.TerminationGracePeriodSeconds, pod.Name)
		i := slices.IndexFunc(pod.Spec.Containers, func(container corev1.Container) bool { return container.Name == "medusa" })
		require.NotEqual(t, -1, i, "%s has no medusa container", pod.Name)
		lifecycle := pod.Spec.Containers[i].Lifecycle
		if enabled {
			require.NotNil(t, lifecycle, pod.Name)
			require.NotNil(t, lifecycle.PreStop, pod.Name)
			require.NotNil(t, lifecycle.PreStop.Exec, pod.Name)
			require.Contains(t, lifecycle.PreStop.Exec.Command, medusapkg.DefaultPitrSpoolDir, pod.Name)
		} else if lifecycle != nil {
			require.Nil(t, lifecycle.PreStop, pod.Name)
		}
	}
}

func (s *medusaPitrTest) pods(key framework.ClusterKey) ([]corev1.Pod, error) {
	dc := &cassdcapi.CassandraDatacenter{}
	if err := s.f.Get(s.ctx, key, dc); err != nil {
		return nil, err
	}
	list := &corev1.PodList{}
	err := s.f.List(s.ctx, key, list, client.MatchingLabels(dc.GetDatacenterLabels()))
	slices.SortFunc(list.Items, func(a, b corev1.Pod) int { return strings.Compare(a.Name, b.Name) })
	return list.Items, err
}

func (s *medusaPitrTest) readyPods(t *testing.T, dc framework.ClusterKey) []corev1.Pod {
	t.Helper()
	var pods []corev1.Pod
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		pods, err = s.pods(dc)
		require.NoError(c, err)
		require.Len(c, pods, pitrNodeCount)
		for _, pod := range pods {
			require.True(c, pitrPodReady(pod), "%s is not ready", pod.Name)
		}
	}, polling.datacenterReady.timeout, polling.medusaPitrArchive.interval)
	return pods
}

func pitrPodReady(pod corev1.Pod) bool {
	return pod.DeletionTimestamp.IsZero() && slices.ContainsFunc(pod.Status.Conditions, func(condition corev1.PodCondition) bool {
		return condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue
	})
}

func (s *medusaPitrTest) warningEvents(dc framework.ClusterKey, reason string) ([]corev1.Event, error) {
	events := &corev1.EventList{}
	err := s.f.List(s.ctx, dc, events, client.MatchingFields{"reason": reason})
	return events.Items, err
}

func failedMedusaHook(events []corev1.Event, pod corev1.Pod) bool {
	return slices.ContainsFunc(events, func(event corev1.Event) bool {
		return event.Type == corev1.EventTypeWarning && event.InvolvedObject.Kind == "Pod" &&
			event.InvolvedObject.UID == pod.UID && event.InvolvedObject.FieldPath == "spec.containers{medusa}"
	})
}

func (s *medusaPitrTest) requireNoFailedHooks(t *testing.T, pods []corev1.Pod) {
	t.Helper()
	events, err := s.warningEvents(s.source, "FailedPreStopHook")
	require.NoError(t, err)
	for _, pod := range pods {
		require.False(t, failedMedusaHook(events, pod), "FailedPreStopHook on %s (UID %s)", pod.Name, pod.UID)
	}
}

// pitrGetPod reads a StatefulSet ordinal by name. A terminating pod and its
// replacement share the name but have different UIDs.
func (s *medusaPitrTest) pitrGetPod(dc framework.ClusterKey, name string) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	err := s.f.Get(s.ctx, pitrJobKey(dc, name), pod)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	return pod, err
}
