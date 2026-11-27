package e2e

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cassdcapi "github.com/k8ssandra/cass-operator/apis/cassandra/v1beta1"
	medusa "github.com/k8ssandra/k8ssandra-operator/apis/medusa/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const pitrFaultGateAnnotation = "e2e.k8ssandra.io/pitr-storage-fault"

// installStorageFaultGate isolates two transitions in this fixture. It lets the
// test interrupt storage after restore preparation and recover it before any
// replacement Pod can start, regardless of the operator's reconcile cadence.
func (s *medusaPitrTest) installStorageFaultGate(t *testing.T) {
	t.Helper()
	key := pitrJobKey(s.source, s.source.Namespace+"-storage-fault")
	const denied = "PITR e2e is arranging the Silo outage"
	phase := fmt.Sprintf("oldObject.metadata.annotations[%q]", pitrFaultGateAnnotation)
	policy := &admissionv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name},
		Spec: admissionv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(admissionv1.Fail),
			MatchConstraints: &admissionv1.MatchResources{
				ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{
					Operations: []admissionv1.OperationType{admissionv1.Update},
					Rule: admissionv1.Rule{
						APIGroups: []string{cassdcapi.GroupVersion.Group}, APIVersions: []string{cassdcapi.GroupVersion.Version},
						Resources: []string{"cassandradatacenters"},
					},
				}}},
			},
			MatchConditions: []admissionv1.MatchCondition{{
				Name:       "fixture-datacenter",
				Expression: fmt.Sprintf("object.metadata.namespace == %q && object.metadata.name == %q", s.source.Namespace, s.source.Name),
			}},
			Variables: []admissionv1.Variable{
				{Name: "wasStopped", Expression: "has(oldObject.spec.stopped) && oldObject.spec.stopped"},
				{Name: "isStopped", Expression: "has(object.spec.stopped) && object.spec.stopped"},
			},
			Validations: []admissionv1.Validation{{
				Expression: fmt.Sprintf(`!has(oldObject.metadata.annotations) ||
					!(%q in oldObject.metadata.annotations) ||
					variables.wasStopped == variables.isStopped ||
					(%s == "hold-stop" && !variables.isStopped) ||
					(%s == "hold-start" && variables.isStopped)`, pitrFaultGateAnnotation, phase, phase),
				Message: denied,
			}},
		},
	}
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name},
		Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: policy.Name, ValidationActions: []admissionv1.ValidationAction{admissionv1.Deny},
		},
	}
	require.NoError(t, s.f.Create(s.ctx, key, policy))
	t.Cleanup(func() { s.deleteObject(t, key, policy) })
	require.NoError(t, s.f.Create(s.ctx, key, binding))
	t.Cleanup(func() { s.deleteObject(t, key, binding) })
	s.setStorageFaultGate(t, "hold-stop")
	t.Cleanup(func() { s.setStorageFaultGate(t, "") })
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		dc := &cassdcapi.CassandraDatacenter{}
		require.NoError(c, s.f.Get(s.ctx, s.source, dc))
		patch := client.MergeFrom(dc.DeepCopy())
		dc.Spec.Stopped = true
		err := s.f.Patch(s.ctx, dc, patch, s.source, client.DryRunAll)
		require.ErrorContains(c, err, denied)
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
}

func (s *medusaPitrTest) setStorageFaultGate(t *testing.T, phase string) {
	t.Helper()
	dc := &cassdcapi.CassandraDatacenter{}
	require.NoError(t, s.f.Get(s.ctx, s.source, dc))
	before := dc.DeepCopy()
	annotations := dc.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if phase == "" {
		delete(annotations, pitrFaultGateAnnotation)
	} else {
		annotations[pitrFaultGateAnnotation] = phase
	}
	dc.SetAnnotations(annotations)
	require.NoError(t, s.f.Patch(s.ctx, dc, client.MergeFrom(before), s.source))
}

func (s *medusaPitrTest) storageUnavailable(t *testing.T) {
	oldPods := s.readyPods(t, s.source)
	s.installStorageFaultGate(t)
	watch := s.watchPods(t, s.source)
	job := s.startRestore(t, s.source, "pitr-storage-outage", s.target, "")
	stopping := false
	t.Cleanup(func() {
		if stopping {
			// Resume the prepared job before its ordinary job cleanup.
			// This cleanup runs after Silo is resumed.
			s.setStorageFaultGate(t, "")
			s.requireRestoreFinished(t, s.source, job)
		}
	})
	prepared, err := s.waitForRestore(s.source, job, func(job *medusa.MedusaRestoreJob) bool {
		return job.Status.RestorePrepared || !job.Status.FinishTime.IsZero()
	})
	require.NoError(t, err)
	require.True(t, prepared.Status.RestorePrepared, prepared.Status.Message)
	require.True(t, prepared.Status.DatacenterStopped.IsZero())

	archivedBeforeOutage, err := s.objects()
	require.NoError(t, err)
	silo := s.siloPod(t)
	require.NoError(t, s.signalSilo(silo, "STOP"))
	paused := true
	t.Cleanup(func() {
		if paused {
			require.NoError(t, s.signalSilo(silo, "CONT"))
		}
	})
	// Freeze the server process so that existing as well as new connections
	// cannot upload. The Silo pod's emptyDir and bucket survive the outage.
	offlineCtx, cancel := context.WithTimeout(s.ctx, 3*time.Second)
	defer cancel()
	_, err = s.storageExec(offlineCtx, "mc", "ls", "pitr/"+s.bucket)
	require.Error(t, err, "Silo remained reachable")
	require.ErrorIs(t, offlineCtx.Err(), context.DeadlineExceeded)

	require.NoError(t, s.writeChurn(s.ctx, oldPods[0].Name, "ALL", pitrChurnBatches))
	var pending map[string]map[string]time.Duration
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pending, err = s.spoolSnapshot(oldPods)
		require.NoError(c, err)
		for _, pod := range oldPods {
			archived := objectsForPod(archivedBeforeOutage, pod.Name)
			for name := range archived {
				delete(pending[pod.Name], name)
			}
			require.NotEmpty(c, pending[pod.Name], "%s needs a pending upload to hold its preStop hook", pod.Name)
		}
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)

	s.setStorageFaultGate(t, "hold-start")
	stopping = true
	s.checkTermination(t, watch, oldPods, true)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		pods, err := s.pods(s.source)
		require.NoError(c, err)
		require.Empty(c, pods, "the DC must be stopped before Silo is restored")
		dc := &cassdcapi.CassandraDatacenter{}
		require.NoError(c, s.f.Get(s.ctx, s.source, dc))
		require.True(c, dc.Spec.Stopped)
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)

	require.NoError(t, s.signalSilo(silo, "CONT"))
	paused = false
	var beforeRestart map[string]pitrObject
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		beforeRestart, err = s.objects()
		require.NoError(c, err)
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
	for pod, files := range pending {
		listed := objectsForPod(beforeRestart, pod)
		for name := range files {
			require.NotContains(t, listed, name, "%s/%s unexpectedly uploaded during the outage", pod, name)
		}
	}

	s.setStorageFaultGate(t, "")
	stopping = false
	s.requireRestoreFinished(t, s.source, job)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		objects, err := s.objects()
		require.NoError(c, err)
		for pod, files := range pending {
			listed := objectsForPod(objects, pod)
			for name := range files {
				require.Contains(c, listed, name, "%s did not upload its pre-restore spool object after restart", pod)
			}
		}
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
	// Assert the kubelet event after verifying recovery, even if the node's
	// runtime does not report the interrupted exec as a hook failure.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		events, err := s.warningEvents(s.source, "FailedPreStopHook")
		require.NoError(c, err)
		for _, pod := range oldPods {
			require.True(c, failedMedusaHook(events, pod), "kubelet did not record a Medusa FailedPreStopHook on %s", pod.Name)
		}
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
	s.verifyRows(t, s.source, clusterName, pitrTargetRows())
}

func (s *medusaPitrTest) deletePodDuringWrites(t *testing.T) {
	pods := s.readyPods(t, s.source)
	oldPod := pods[0]
	watch := s.watchPods(t, s.source)
	writerCtx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	wrote := make(chan struct{}, 1)
	writeErr := make(chan error, 1)
	done := make(chan struct{})
	var deleting atomic.Bool
	go func() {
		defer close(done)
		for writerCtx.Err() == nil {
			if err := s.writeChurn(writerCtx, pods[1].Name, "ONE", pitrChurnBatches/2); err != nil {
				// The workload rewrites the same filler rows. A Cassandra
				// timeout can be retried. While a replica is down, the
				// authenticated coordinator can also lose its auth quorum.
				message := err.Error()
				authQuorumUnavailable := deleting.Load() && strings.Contains(message, "Unable to perform auth") &&
					strings.Contains(message, "Cannot achieve consistency level LOCAL_QUORUM")
				if strings.Contains(message, "WriteTimeout:") || strings.Contains(message, "OperationTimedOut:") || authQuorumUnavailable {
					continue
				}
				if writerCtx.Err() == nil {
					writeErr <- err
				}
				return
			}
			select {
			case wrote <- struct{}{}:
			default:
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	select {
	case <-wrote:
	case err := <-writeErr:
		require.NoError(t, err)
	case <-time.After(polling.medusaPitrArchive.timeout):
		t.Fatal("no workload writes completed")
	}

	var spool map[string]time.Duration
	observedBeforeReady := make(map[string]pitrObject)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		objects, err := s.objects()
		require.NoError(c, err)
		observedBeforeReady = objectsForPod(objects, oldPod.Name)
		spool, err = s.spool(oldPod.Name)
		require.NoError(c, err)
		for name := range spool {
			if _, uploaded := observedBeforeReady[name]; !uploaded {
				return
			}
		}
		c.Errorf("waiting for %s to spool an unuploaded segment while the workload runs", oldPod.Name)
	}, polling.medusaPitrArchive.timeout, polling.medusaPitrArchive.interval)
	client, err := s.f.GetClient(s.source.K8sContext)
	require.NoError(t, err)
	// Use the normal Pod DELETE API with a UID precondition so that a new
	// incarnation of the same StatefulSet ordinal cannot be deleted.
	deleting.Store(true)
	require.NoError(t, client.Delete(s.ctx, &oldPod, pitrDeleteOptions(oldPod)))

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		old, err := watch.observation(oldPod.UID)
		require.NoError(c, err)
		if !old.gone.IsZero() {
			cancel()
		}
		objects, err := s.objects()
		require.NoError(c, err)
		uploaded := objectsForPod(objects, oldPod.Name)
		pod, err := s.pitrGetPod(s.source, oldPod.Name)
		require.NoError(c, err)
		require.NotNil(c, pod)
		if !pitrPodReady(*pod) {
			observation, err := watch.observation(pod.UID)
			require.NoError(c, err)
			if !observation.ready || pod.UID == oldPod.UID {
				for name, object := range uploaded {
					observedBeforeReady[name] = object
				}
			}
		}
		require.NotEqual(c, oldPod.UID, pod.UID)
		require.True(c, pitrPodReady(*pod))
		for _, condition := range pod.Status.Conditions {
			if condition.Type != corev1.PodReady {
				continue
			}
			for name := range spool {
				object, found := uploaded[name]
				require.True(c, found, "replacement was Ready with %s missing", name)
				_, seen := observedBeforeReady[name]
				require.True(c, seen || object.LastModified.Before(condition.LastTransitionTime.Time),
					"%s was uploaded at %s; replacement Ready at %s", name, object.LastModified, condition.LastTransitionTime)
			}
		}
	}, polling.medusaRestoreDone.timeout, polling.medusaPitrArchive.interval)
	cancel()
	<-done
	select {
	case err := <-writeErr:
		assert.NoError(t, err)
	default:
	}

	s.checkTermination(t, watch, []corev1.Pod{oldPod}, false)
	observation, err := watch.observation(oldPod.UID)
	require.NoError(t, err)
	require.False(t, observation.cassandraExited.IsZero(), "old Cassandra termination was not observed")
	require.False(t, observation.medusaExited.IsZero(), "old Medusa termination was not observed")
	require.False(t, observation.medusaExited.Before(observation.cassandraExited), "Medusa exited before Cassandra")
	s.requireNoFailedHooks(t, []corev1.Pod{oldPod})
	checkDatacenterReady(t, s.ctx, s.source, s.f)
}
