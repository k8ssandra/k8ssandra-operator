package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	medusapkg "github.com/k8ssandra/k8ssandra-operator/pkg/medusa"
	"github.com/k8ssandra/k8ssandra-operator/test/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type pitrObject struct {
	Key          string    `json:"key"`
	LastModified time.Time `json:"lastModified"`
	Size         int64     `json:"size"`
}

var pitrSegmentName = regexp.MustCompile(`^CommitLog-[0-9]+-([0-9]+)\.log$`)

func pitrSegmentID(key string) (int64, error) {
	name := pitrSegmentName.FindStringSubmatch(path.Base(key))
	if len(name) == 0 {
		return 0, fmt.Errorf("invalid commitlog segment name %q", key)
	}
	return strconv.ParseInt(name[1], 10, 64)
}

// pitrKubectl runs cancellable exec and log requests without logging database credentials.
func pitrKubectl(ctx context.Context, key framework.ClusterKey, stdin string, args ...string) (string, error) {
	timeout := 30 * time.Second
	if stdin != "" {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", append([]string{
		"--kubeconfig", kubeconfigFile, "--context", key.K8sContext, "--namespace", key.Namespace,
	}, args...)...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return string(out), fmt.Errorf("kubectl %s in %s: %w: %s%s", args[0], key, err, out, stderr.String())
	}
	return string(out), nil
}

func (s *medusaPitrTest) exec(ctx context.Context, dc framework.ClusterKey, pod, container string, args ...string) (string, error) {
	return pitrKubectl(ctx, pitrJobKey(dc, pod), "", append([]string{"exec", pod, "-c", container, "--"}, args...)...)
}

func (s *medusaPitrTest) logs(ctx context.Context, dc framework.ClusterKey, pod, container string, previous bool) (string, error) {
	args := []string{"logs", pod, "-c", container}
	if previous {
		args = append(args, "--previous")
	}
	return pitrKubectl(ctx, pitrJobKey(dc, pod), "", args...)
}

// preparePitrStorage keeps Silo as a child of the container shell so the
// outage can pause it without deleting the pod or its emptyDir bucket. Linux
// does not deliver SIGSTOP from within a PID namespace to its PID 1.
func (s *medusaPitrTest) preparePitrStorage(t *testing.T) {
	t.Helper()
	key := pitrJobKey(s.toolbox, "silo")
	deployment := &appsv1.Deployment{}
	require.NoError(t, s.f.Get(s.ctx, key, deployment))
	patch := client.MergeFrom(deployment.DeepCopy())
	containers := deployment.Spec.Template.Spec.Containers
	require.Len(t, containers, 1)
	require.Equal(t, "silo", containers[0].Name)
	require.Len(t, containers[0].Args, 1)
	const wrapper = " & wait"
	if !strings.HasSuffix(containers[0].Args[0], wrapper) {
		containers[0].Args[0] += wrapper
	}
	template := &deployment.Spec.Template
	if template.Annotations == nil {
		template.Annotations = make(map[string]string)
	}
	template.Annotations["e2e.k8ssandra.io/pitr-fixture"] = s.source.Namespace
	require.NoError(t, s.f.Patch(s.ctx, deployment, patch, key))
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.f.Get(s.ctx, key, deployment))
		require.Equal(c, deployment.Generation, deployment.Status.ObservedGeneration)
		require.Equal(c, int32(1), deployment.Status.UpdatedReplicas)
		require.Equal(c, int32(1), deployment.Status.AvailableReplicas)
		require.Equal(c, int32(1), deployment.Status.Replicas)
	}, polling.medusaReady.timeout, polling.medusaReady.interval)

	// Its replacement is empty. Recreate the fixture bucket before any backup.
	jobKey := pitrJobKey(s.toolbox, "setup-silo")
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: jobKey.Name, Namespace: jobKey.Namespace}}
	s.deleteObject(t, jobKey, job)
	require.Eventually(t, func() bool {
		return apierrors.IsNotFound(s.f.Get(s.ctx, jobKey, job))
	}, polling.medusaReady.timeout, polling.medusaPitrArchive.interval)
	require.NoError(t, s.f.CreateMedusaBucket(s.toolbox.Namespace))
}

func (s *medusaPitrTest) startStorageClient(t *testing.T) {
	t.Helper()
	toolbox := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: s.toolbox.Name, Namespace: s.toolbox.Namespace},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:    "mc",
				Image:   "docker.io/pgsty/mc",
				Command: []string{"/bin/sh", "-c", "exec tail -f /dev/null"},
				Env: []corev1.EnvVar{{
					// Credentials match the Silo setup job and medusa-bucket-key fixture.
					Name: "MC_HOST_pitr", Value: "http://k8ssandra:k8ssandra@silo-service:9000",
				}},
			}},
		},
	}
	require.NoError(t, s.f.Create(s.ctx, s.toolbox, toolbox))
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.f.Get(s.ctx, s.toolbox, toolbox))
		require.True(c, pitrPodReady(*toolbox))
		_, err := s.storageExec(s.ctx, "mc", "ls", "pitr/"+s.bucket)
		require.NoError(c, err)
	}, polling.medusaReady.timeout, polling.medusaReady.interval)
}

func (s *medusaPitrTest) removeStorageClient(t *testing.T) {
	t.Helper()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: s.toolbox.Name, Namespace: s.toolbox.Namespace}}
	s.deleteObject(t, s.toolbox, pod)
}

func (s *medusaPitrTest) storageExec(ctx context.Context, args ...string) (string, error) {
	return s.exec(ctx, s.toolbox, s.toolbox.Name, "mc", args...)
}

func (s *medusaPitrTest) objects() (map[string]pitrObject, error) {
	out, err := s.storageExec(s.ctx, "mc", "--json", "ls", "--recursive", "pitr/"+s.bucket+"/"+s.prefix+"/")
	if err != nil {
		return nil, err
	}
	objects := make(map[string]pitrObject)
	decoder := json.NewDecoder(strings.NewReader(out))
	for decoder.More() {
		var object pitrObject
		if err := decoder.Decode(&object); err != nil {
			return nil, fmt.Errorf("parse Silo object listing: %w", err)
		}
		// mc reports keys relative to the requested prefix.
		if strings.Contains(object.Key, "/commitlogs/") && object.Size > 0 {
			objects[object.Key] = object
		}
	}
	return objects, nil
}

func objectsForPod(objects map[string]pitrObject, pod string) map[string]pitrObject {
	files := make(map[string]pitrObject)
	for key, object := range objects {
		node, file, ok := strings.Cut(key, "/commitlogs/")
		if ok && strings.SplitN(path.Base(node), ".", 2)[0] == pod {
			files[file] = object
		}
	}
	return files
}

func (s *medusaPitrTest) spool(pod string) (map[string]time.Duration, error) {
	out, err := s.exec(s.ctx, s.source, pod, "medusa", "python3", "-c", `
import os, sys, time
try:
    with os.scandir(sys.argv[1]) as entries:
        for entry in entries:
            try:
                if entry.is_file(follow_symlinks=False):
                    print(entry.name, max(0, time.time() - entry.stat(follow_symlinks=False).st_mtime))
            except FileNotFoundError:
                pass  # A successful upload may unlink this entry while we list it.
except FileNotFoundError:
    pass
`, medusapkg.DefaultPitrSpoolDir)
	if err != nil {
		return nil, err
	}
	spool := make(map[string]time.Duration)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		name, age, ok := strings.Cut(line, " ")
		if !ok {
			return nil, fmt.Errorf("invalid spool entry %q", line)
		}
		seconds, err := strconv.ParseFloat(age, 64)
		if err != nil {
			return nil, err
		}
		spool[name] = time.Duration(seconds * float64(time.Second))
	}
	return spool, nil
}

// exposeStorageToOtherPlane lets the second kind cluster use the source Silo
// instance. The two clusters share the kind Docker network and have separate
// ClusterIPs and Silo buckets by default.
func (s *medusaPitrTest) exposeStorageToOtherPlane(t *testing.T) (string, int) {
	t.Helper()
	key := pitrJobKey(s.toolbox, "pitr-silo-nodeport")
	host := s.siloPod(t).Status.HostIP
	require.NotEmpty(t, host)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "silo"},
			Ports:    []corev1.ServicePort{{Port: 9000, TargetPort: intstr.FromInt(9000)}},
		},
	}
	require.NoError(t, s.f.Create(s.ctx, key, service))
	t.Cleanup(func() { s.deleteObject(t, key, service) })
	require.NotZero(t, service.Spec.Ports[0].NodePort)
	return host, int(service.Spec.Ports[0].NodePort)
}

// truncateSegment copies the first half back to the same S3 key and keeps the
// original in the toolbox for cleanup. The workload and restore use real Medusa.
func (s *medusaPitrTest) truncateSegment(t *testing.T, object pitrObject) {
	t.Helper()
	require.Greater(t, object.Size, int64(1))
	key := "pitr/" + s.bucket + "/" + s.prefix + "/" + object.Key
	original, err := s.storageExec(s.ctx, "/bin/sh", "-ec", `
file=$(mktemp)
mc cp "$1" "$file" >&2
printf '%s\n' "$file"
`, "sh", key)
	require.NoError(t, err)
	original = strings.TrimSpace(original)
	require.NotEmpty(t, original)
	t.Cleanup(func() {
		_, err := s.storageExec(s.ctx, "/bin/sh", "-ec", `mc cp "$1" "$2" && rm -f "$1"`, "sh", original, key)
		require.NoError(t, err)
	})
	_, err = s.storageExec(s.ctx, "/bin/sh", "-ec", `
half=$(mktemp)
trap 'rm -f "$half"' EXIT
head -c "$1" "$2" > "$half"
mc cp "$half" "$3"
`, "sh", strconv.FormatInt(object.Size/2, 10), original, key)
	require.NoError(t, err)
	objects, err := s.objects()
	require.NoError(t, err)
	require.Equal(t, object.Size/2, objects[object.Key].Size)
}

func (s *medusaPitrTest) siloPod(t *testing.T) corev1.Pod {
	t.Helper()
	pods := &corev1.PodList{}
	require.NoError(t, s.f.List(s.ctx, s.toolbox, pods, client.MatchingLabels{"app": "silo"}))
	require.Len(t, pods.Items, 1)
	return pods.Items[0]
}

func (s *medusaPitrTest) signalSilo(pod corev1.Pod, signal string) error {
	key := framework.NewClusterKey(s.source.K8sContext, pod.Namespace, pod.Name)
	// The Silo fixture runs only the server and its shell in this container.
	_, err := s.exec(s.ctx, key, pod.Name, "silo", "/bin/bash", "-ec",
		`for proc in /proc/[0-9]*/comm; do
  read -r name < "$proc" || continue
  if [[ "$name" == silo ]]; then
    pid=${proc%/comm}
    kill -"$1" "${pid##*/}"
    exit
  fi
done
exit 1`, "bash", signal)
	return err
}

func pitrDeleteOptions(pod corev1.Pod) client.DeleteOption {
	return client.Preconditions{UID: ptr.To(pod.UID)}
}
