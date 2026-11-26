package e2e

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pitrPodObservation struct {
	name              string
	deletionRequested time.Time
	gone              time.Time
	ready             bool
	cassandraExited   time.Time
	medusaExited      time.Time
}

type pitrPodWatch struct {
	mu   sync.Mutex
	pods map[types.UID]pitrPodObservation
	err  error
}

// watchPods watches from a list resource version to capture short-lived pod
// states, including deletion of one UID before replacement by another.
func (s *medusaPitrTest) watchPods(t *testing.T, dcKey framework.ClusterKey) *pitrPodWatch {
	t.Helper()
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigFile},
		&clientcmd.ConfigOverrides{CurrentContext: dcKey.K8sContext},
	)
	restConfig, err := config.ClientConfig()
	require.NoError(t, err)
	kube, err := kubernetes.NewForConfig(restConfig)
	require.NoError(t, err)
	dc := &cassdcapi.CassandraDatacenter{}
	require.NoError(t, s.f.Get(s.ctx, dcKey, dc))
	api := kube.CoreV1().Pods(dcKey.Namespace)
	opts := metav1.ListOptions{LabelSelector: labels.Set(dc.GetDatacenterLabels()).String()}
	current, err := api.List(s.ctx, opts)
	require.NoError(t, err)
	opts.ResourceVersion = current.ResourceVersion
	ctx, cancel := context.WithCancel(s.ctx)
	t.Cleanup(cancel)
	stream, err := api.Watch(ctx, opts)
	require.NoError(t, err)
	t.Cleanup(stream.Stop)
	observed := &pitrPodWatch{pods: make(map[types.UID]pitrPodObservation)}
	go observed.record(stream.ResultChan())
	return observed
}

func (w *pitrPodWatch) record(events <-chan watch.Event) {
	for event := range events {
		if event.Type == watch.Bookmark {
			continue
		}
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			w.mu.Lock()
			w.err = fmt.Errorf("watching Cassandra pods: %s %v", event.Type, event.Object)
			w.mu.Unlock()
			return
		}
		w.mu.Lock()
		observation := w.pods[pod.UID]
		observation.name = pod.Name
		observation.ready = observation.ready || pitrPodReady(*pod)
		if observation.deletionRequested.IsZero() && pod.DeletionGracePeriodSeconds != nil && !pod.DeletionTimestamp.IsZero() {
			observation.deletionRequested = pod.DeletionTimestamp.Add(-time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second)
		}
		if !pod.DeletionTimestamp.IsZero() {
			for _, status := range pod.Status.ContainerStatuses {
				if status.State.Terminated == nil {
					continue
				}
				switch status.Name {
				case "cassandra":
					observation.cassandraExited = status.State.Terminated.FinishedAt.Time
				case "medusa":
					observation.medusaExited = status.State.Terminated.FinishedAt.Time
				}
			}
		}
		if event.Type == watch.Deleted {
			observation.gone = time.Now()
		}
		w.pods[pod.UID] = observation
		w.mu.Unlock()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err == nil {
		w.err = fmt.Errorf("Cassandra pod watch closed")
	}
}

func (w *pitrPodWatch) observation(uid types.UID) (pitrPodObservation, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pods[uid], w.err
}

func (w *pitrPodWatch) readyReplacement(oldPod corev1.Pod) (types.UID, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for uid, observation := range w.pods {
		if uid != oldPod.UID && observation.name == oldPod.Name && observation.ready {
			return uid, w.err
		}
	}
	return "", w.err
}

func (s *medusaPitrTest) checkTermination(t *testing.T, watch *pitrPodWatch, pods []corev1.Pod, exhaustedGrace bool) {
	t.Helper()
	// A stop can spend time draining before deletion is requested. The
	// outage path then waits through the full grace period by design.
	timeout := polling.datacenterUpdating.timeout + pitrGrace
	for _, pod := range pods {
		var observation pitrPodObservation
		require.EventuallyWithT(t, func(c *assert.CollectT) {
			var err error
			observation, err = watch.observation(pod.UID)
			require.NoError(c, err)
			require.False(c, observation.gone.IsZero(), "%s deletion was not observed", pod.Name)
			require.False(c, observation.deletionRequested.IsZero(), "%s deletion request was not observed", pod.Name)
		}, timeout, polling.medusaPitrArchive.interval)
		duration := observation.gone.Sub(observation.deletionRequested)
		t.Logf("%s (UID %s) deletion to disappearance: %s (grace period %s)", pod.Name, pod.UID, duration.Round(time.Millisecond), pitrGrace)
		assert.Positive(t, duration)
		if exhaustedGrace {
			// The API timestamps have whole-second resolution.
			assert.GreaterOrEqual(t, duration, pitrGrace-time.Second, "%s exited before exhausting its grace period", pod.Name)
		} else {
			assert.Less(t, duration, pitrGrace, "%s exhausted its termination grace period", pod.Name)
		}
	}
}

func (s *medusaPitrTest) spoolSnapshot(pods []corev1.Pod) (map[string]map[string]time.Duration, error) {
	snapshot := make(map[string]map[string]time.Duration, len(pods))
	for _, pod := range pods {
		files, err := s.spool(pod.Name)
		if err != nil {
			return nil, err
		}
		snapshot[pod.Name] = files
	}
	return snapshot, nil
}

func pitrRestoreInitStarted(pod corev1.Pod) time.Time {
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name == "medusa-restore" {
			if status.State.Running != nil {
				return status.State.Running.StartedAt.Time
			}
			if status.State.Terminated != nil {
				return status.State.Terminated.StartedAt.Time
			}
		}
	}
	return time.Time{}
}

func pitrCassandraStarted(pod corev1.Pod) time.Time {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "cassandra" && status.State.Running != nil {
			return status.State.Running.StartedAt.Time
		}
	}
	return time.Time{}
}

func pitrUploadPrecededInit(uploaded, initStarted time.Time, seenWhileOld bool) bool {
	// Kubernetes and object storage can report the same second for either order.
	return seenWhileOld || uploaded.Before(initStarted)
}

func TestPitrUploadPrecededInit(t *testing.T) {
	start := time.Unix(100, 0)
	for _, tc := range []struct {
		name         string
		offset       time.Duration
		seenWhileOld bool
		want         bool
	}{
		{name: "before", offset: -time.Second, want: true},
		{name: "equal without earlier observation"},
		{name: "equal with earlier observation", seenWhileOld: true, want: true},
		{name: "after", offset: time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, pitrUploadPrecededInit(start.Add(tc.offset), start, tc.seenWhileOld))
		})
	}
}

func (s *medusaPitrTest) restoreInPlace(t *testing.T) {
	oldPods := s.readyPods(t, s.source)
	watch := s.watchPods(t, s.source)
	var before map[string]pitrObject
	var spool map[string]map[string]time.Duration
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var err error
		before, err = s.objects()
		require.NoError(c, err)
		spool, err = s.spoolSnapshot(oldPods)
		require.NoError(c, err)
		allHavePending := true
		for _, pod := range oldPods {
			listed := objectsForPod(before, pod.Name)
			pending := false
			for name := range spool[pod.Name] {
				if _, found := listed[name]; !found {
					pending = true
					break
				}
			}
			allHavePending = allHavePending && pending
		}
		if !allHavePending {
			require.NoError(c, s.writeChurn(s.ctx, oldPods[0].Name, "ALL", pitrChurnBatches))
			c.Errorf("waiting to capture an unuploaded segment in each spool")
		}
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)

	job := s.startRestore(t, s.source, "pitr-in-place", s.target, "")
	initial := make(map[string]map[string]pitrObject, len(oldPods))
	replacements := make(map[string]types.UID, len(oldPods))
	seenWhileOld := make(map[string]map[string]pitrObject, len(oldPods))
	for _, oldPod := range oldPods {
		seenWhileOld[oldPod.Name] = objectsForPod(before, oldPod.Name)
	}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		objects, err := s.objects()
		require.NoError(c, err)
		for _, oldPod := range oldPods {
			if _, observed := initial[oldPod.Name]; observed {
				continue
			}
			pod, err := s.pitrGetPod(s.source, oldPod.Name)
			require.NoError(c, err)
			if pod == nil {
				continue
			}
			uploaded := objectsForPod(objects, oldPod.Name)
			if pod.UID == oldPod.UID {
				// These objects were observed before the replacement could run.
				maps.Copy(seenWhileOld[oldPod.Name], uploaded)
				continue
			}
			initStarted := pitrRestoreInitStarted(*pod)
			if initStarted.IsZero() {
				// Bucket listing preceded this pod observation. An init
				// container with no start time may already be running, so
				// use only the S3 timestamps after the old UID is gone.
				continue
			}
			for name := range spool[oldPod.Name] {
				object, found := uploaded[name]
				require.True(c, found, "%s was spooled by %s and is missing at restore init start", name, oldPod.Name)
				_, observedBeforeReplacement := seenWhileOld[oldPod.Name][name]
				require.True(c, pitrUploadPrecededInit(object.LastModified, initStarted, observedBeforeReplacement),
					"%s/%s upload time %s did not precede restore init at %s", oldPod.Name, name, object.LastModified, initStarted)
			}
			maps.DeleteFunc(uploaded, func(name string, object pitrObject) bool {
				_, observedEarlier := seenWhileOld[oldPod.Name][name]
				return !pitrUploadPrecededInit(object.LastModified, initStarted, observedEarlier)
			})
			initial[oldPod.Name] = uploaded
			replacements[oldPod.Name] = pod.UID
		}
		latest := &medusa.MedusaRestoreJob{}
		require.NoError(c, s.f.Get(s.ctx, pitrJobKey(s.source, job.Name), latest))
		require.Empty(c, latest.Status.Message, "restore failed before init containers started")
		require.Len(c, initial, len(oldPods), "waiting for the replacement pods' medusa-restore init containers")
	}, polling.medusaRestoreDone.timeout, polling.medusaPitrArchive.interval)
	s.checkTermination(t, watch, oldPods, false)

	finished := s.requireRestoreFinished(t, s.source, job)
	s.inPlaceStatus = finished.Status
	require.Equal(t, ptr.To(true), finished.Status.RestoreMapping.InPlace)
	require.NotNil(t, finished.Status.RestoreMapping.Pitr)
	require.Equal(t, s.target.Format(time.RFC3339Nano), finished.Status.RestoreMapping.Pitr.TargetTimestamp)
	require.Len(t, finished.Status.RestoreMapping.Pitr.NodeBackups, len(oldPods))
	for _, backup := range finished.Status.RestoreMapping.Pitr.NodeBackups {
		require.Equal(t, s.baseBackup, backup)
	}

	pods := s.readyPods(t, s.source)
	for _, pod := range pods {
		require.Equal(t, replacements[pod.Name], pod.UID, "%s restarted again during restore", pod.Name)
	}
	// Produce new segments so this assertion also exercises a live archiver.
	require.NoError(t, s.writeChurn(s.ctx, pods[0].Name, "ALL", pitrChurnBatches))
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.False(c, time.Now().Before(finished.Status.FinishTime.Add(2*pitrInterval)))
		objects, err := s.objects()
		require.NoError(c, err)
		for _, pod := range pods {
			start := pitrCassandraStarted(pod)
			require.False(c, start.IsZero(), pod.Name)
			added := 0
			for name := range objectsForPod(objects, pod.Name) {
				if _, existed := initial[pod.Name][name]; existed {
					continue
				}
				id, err := pitrSegmentID(name)
				require.NoError(c, err)
				require.Greater(c, id, start.UnixMilli(), "%s uploaded old segment %s after restore init", pod.Name, name)
				added++
			}
			require.Positive(c, added, "waiting for an archived segment from the new timeline on %s", pod.Name)
		}
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
	s.requireNoFailedHooks(t, append(oldPods, pods...))
}

// corruptSegment chooses a sealed object already uploaded before the target,
// with another target-window segment after it on the same node.
func (s *medusaPitrTest) corruptSegment(t *testing.T) {
	oldPods := s.readyPods(t, s.source)
	var corruptPod corev1.Pod
	var segment pitrObject
	for _, pod := range oldPods {
		objects := slices.Collect(maps.Values(objectsForPod(s.targetObjects, pod.Name)))
		if len(objects) < 2 {
			continue
		}
		for _, object := range objects {
			_, err := pitrSegmentID(object.Key)
			require.NoError(t, err)
			require.False(t, object.LastModified.After(s.target))
		}
		slices.SortFunc(objects, func(a, b pitrObject) int {
			aID, _ := pitrSegmentID(a.Key)
			bID, _ := pitrSegmentID(b.Key)
			return cmp.Compare(aID, bID)
		})
		corruptPod, segment = pod, objects[len(objects)-2]
		break
	}
	require.NotEmpty(t, segment.Key, "no non-tail commitlog segment in the target window")
	t.Logf("Truncating %s for %s", segment.Key, corruptPod.Name)
	s.truncateSegment(t, segment)

	dc := &cassdcapi.CassandraDatacenter{}
	require.NoError(t, s.f.Get(s.ctx, s.source, dc))
	watch := s.watchPods(t, s.source)
	previousEvents, err := s.datacenterEvents(dc.UID)
	require.NoError(t, err)
	job := s.startRestore(t, s.source, "pitr-corrupt", s.target, "")
	t.Run("replay_stays_failed_for_five_minutes", func(t *testing.T) {
		s.observeFailedReplay(t, job, corruptPod, path.Base(segment.Key), dc.UID, previousEvents, watch)
	})

	// Release the incomplete restore before submitting the recovery job, even
	// when the replay assertions fail. Each job has its own cleanup.
	key := pitrJobKey(s.source, job.Name)
	s.deleteObject(t, key, job)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.True(c, apierrors.IsNotFound(s.f.Get(s.ctx, key, job)))
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
	recovery := s.startRestore(t, s.source, "pitr-recover-base", time.Time{}, s.baseBackup)
	s.requireRestoreFinished(t, s.source, recovery)
	s.verifyRows(t, s.source, clusterName, map[int]string{1: "base-one", 2: "base-two"})
}

func (s *medusaPitrTest) observeFailedReplay(t *testing.T, job *medusa.MedusaRestoreJob, oldPod corev1.Pod, segment string,
	dcUID types.UID, previousEvents []corev1.Event, podWatch *pitrPodWatch) {
	t.Helper()
	waitForInit := time.Now().Add(polling.medusaRestoreDone.timeout)
	ticker := time.NewTicker(polling.medusaPitrReplayFailure.interval)
	defer ticker.Stop()
	var firstReplacement types.UID
	var deadline time.Time
	logCaptured, recreated, stuckWarning := false, false, false
	for {
		latest := &medusa.MedusaRestoreJob{}
		require.NoError(t, s.f.Get(s.ctx, pitrJobKey(s.source, job.Name), latest))
		require.Empty(t, latest.Status.Message)
		require.True(t, latest.Status.FinishTime.IsZero(), "corrupt restore completed")

		readyUID, err := podWatch.readyReplacement(oldPod)
		require.NoError(t, err)
		require.Empty(t, readyUID, "a replacement for %s reached Ready during the corrupt restore", oldPod.Name)
		pod, err := s.pitrGetPod(s.source, oldPod.Name)
		require.NoError(t, err)
		if pod != nil && pod.UID != oldPod.UID {
			if firstReplacement == "" {
				firstReplacement = pod.UID
			}
			recreated = recreated || pod.UID != firstReplacement
			require.False(t, pitrPodReady(*pod), "corrupt replay pod %s (UID %s) reached Ready", pod.Name, pod.UID)
			if deadline.IsZero() && !pitrRestoreInitStarted(*pod).IsZero() {
				deadline = time.Now().Add(polling.medusaPitrReplayFailure.timeout)
			}
			if !logCaptured {
				for _, container := range []string{"server-system-logger", "cassandra"} {
					for _, previous := range []bool{false, true} {
						log, err := s.logs(s.ctx, s.source, pod.Name, container, previous)
						if err == nil {
							for line := range strings.SplitSeq(log, "\n") {
								// Cassandra 4.1.8 reports this fatal read as a
								// CommitLogReadException naming the corrupt segment.
								if strings.Contains(line, "CommitLogReplayException") ||
									(strings.Contains(line, "CommitLogReadException") && strings.Contains(line, segment)) {
									t.Logf("Captured Cassandra replay error on %s (UID %s) in %s: %s", pod.Name, pod.UID, container, line)
									logCaptured = true
									break
								}
							}
							if logCaptured {
								break
							}
						}
					}
					if logCaptured {
						break
					}
				}
			}
		}

		if !stuckWarning {
			events, err := s.datacenterEvents(dcUID)
			require.NoError(t, err)
			stuckWarning = slices.ContainsFunc(events, func(event corev1.Event) bool {
				// cass-operator 1.33 puts the identifier in the event action.
				stuck := event.Action == "DeletingStuckPod" || event.Reason == "DeletingStuckPod"
				return event.InvolvedObject.Kind == "CassandraDatacenter" && event.InvolvedObject.UID == dcUID &&
					event.Type == corev1.EventTypeWarning && stuck && !slices.ContainsFunc(previousEvents, func(previous corev1.Event) bool {
					return event.UID == previous.UID && event.ResourceVersion == previous.ResourceVersion
				})
			})
		}

		if deadline.IsZero() {
			require.True(t, time.Now().Before(waitForInit), "corrupt restore never started its replacement init container")
		} else if !time.Now().Before(deadline) {
			require.True(t, logCaptured, "no Cassandra replay exception was captured for %s", segment)
			require.True(t, stuckWarning, "no CassandraDatacenter DeletingStuckPod warning")
			require.True(t, recreated, "cass-operator did not recreate the failing ordinal")
			return
		}
		select {
		case <-s.ctx.Done():
			t.Fatal(s.ctx.Err())
		case <-ticker.C:
		}
	}
}

func (s *medusaPitrTest) datacenterEvents(uid types.UID) ([]corev1.Event, error) {
	events := &corev1.EventList{}
	err := s.f.List(s.ctx, s.source, events, client.MatchingFields{"involvedObject.uid": string(uid)})
	return events.Items, err
}
