package kubectl

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type Options struct {
	Namespace  string
	Context    string
	ServerSide bool
}

var logOutput = false

func LogOutput(enabled bool) {
	logOutput = enabled
}

func ApplyKustomize(opts Options, arg interface{}) error {
	return applyInternal(opts, true, arg)
}

func Apply(opts Options, arg interface{}) error {
	return applyInternal(opts, false, arg)
}

func applyInternal(opts Options, kustomize bool, arg interface{}) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "apply")

	if opts.ServerSide {
		cmd.Args = append(cmd.Args, "--server-side", "--force-conflicts")
	}

	if kustomize {
		cmd.Args = append(cmd.Args, "-k")
	} else {
		cmd.Args = append(cmd.Args, "-f")
	}

	if buf, ok := arg.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else if s, ok := arg.(string); ok {
		cmd.Args = append(cmd.Args, s)
	} else {
		return errors.New("Expected arg to be a *bytes.Buffer or a string")
	}

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		fmt.Println(string(output))
	}

	return err
}

func Delete(opts Options, arg interface{}) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "delete", "-f")

	if buf, ok := arg.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else if s, ok := arg.(string); ok {
		cmd.Args = append(cmd.Args, s)
	} else {
		return errors.New("Expected arg to be a *bytes.Buffer or a string")
	}

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		fmt.Println(string(output))
	}

	return err
}

func DeleteByName(opts Options, kind, name string, ignoreNotFound bool) error {
	cmd := exec.Command("kubectl")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}
	cmd.Args = append(cmd.Args, "delete", kind, name)
	if ignoreNotFound {
		cmd.Args = append(cmd.Args, "--ignore-not-found")
	}
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if logOutput || err != nil {
		fmt.Println(string(output))
	}
	return err
}

func DeleteAllOf(opts Options, kind string) error {
	cmd := exec.Command("kubectl")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	} else {
		return errors.New("Namespace is required for delete --all")
	}
	cmd.Args = append(cmd.Args, "delete", kind, "--all")
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if logOutput || err != nil {
		fmt.Println(string(output))
	}
	return err
}

func WaitForCondition(opts Options, condition string, args ...string) error {

	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "wait", "--for", "condition="+condition)
	cmd.Args = append(cmd.Args, args...)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		fmt.Println(string(output))
	}

	return err
}

// Exec executes a command against a Cassandra pod and the cassandra container in
// particular. This does not currently handle pipes.
func Exec(opts Options, pod string, args ...string) (string, error) {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "exec", "-i", pod, "-c", "cassandra", "--")
	cmd.Args = append(cmd.Args, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	fmt.Println(cmd)

	err := cmd.Run()

	if logOutput {
		fmt.Println(stderr.String())
		fmt.Println(stdout.String())
	}

	if err != nil {
		return stderr.String(), err
	}
	return stdout.String(), nil
}

func PortForward(opts Options, ctx context.Context, target string, externalPort, internalPort int) error {
	cmd := exec.CommandContext(ctx, "kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "port-forward", target, fmt.Sprintf("%v:%v", externalPort, internalPort))

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		fmt.Println(string(output))
	}

	return err
}

type ClusterInfoOptions struct {
	Options

	Namespaces []string

	OutputDirectory string
}

func DumpClusterInfo(opts ClusterInfoOptions) error {
	cmd := exec.Command("kubectl", "cluster-info", "dump")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespaces) > 0 {
		cmd.Args = append(cmd.Args, "--namespaces", strings.Join(opts.Namespaces, ","))
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "-o", "yaml")

	dir, err := filepath.Abs(opts.OutputDirectory)
	if err != nil {
		return err
	}

	cmd.Args = append(cmd.Args, "--output-directory", dir)

	output, err := cmd.CombinedOutput()

	if logOutput || err != nil {
		fmt.Println(string(output))
	}

	return err
}

func Get(opts Options, args ...string) (string, error) {
	cmd := exec.Command("kubectl", "get")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	fmt.Println(cmd)

	err := cmd.Run()

	if logOutput || err != nil {
		fmt.Println(stderr.String())
		fmt.Println(stdout.String())
	}

	if err != nil {
		return stdout.String() + stderr.String(), err
	}
	return stdout.String(), nil
}

func RolloutStatus(ctx context.Context, opts Options, kind, name string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "status")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}
	cmd.Args = append(cmd.Args, kind, name)
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if logOutput || err != nil {
		fmt.Println(string(output))
	}
	return err
}

func StatefulSetReadyReplicas(ctx context.Context, opts Options, namespace, name string) (int, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "statefulset", name, "-o", "jsonpath='{.status.readyReplicas}'")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	cmd.Args = append(cmd.Args, "-n", namespace)

	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get ready replicas: %s", string(output))
	}
	outputStr := strings.Trim(string(output), "'")
	return strconv.Atoi(outputStr)
}

func JobSuccess(ctx context.Context, opts Options, namespace, name string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "job", name, "-o", "jsonpath='{.status.succeeded}'")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	cmd.Args = append(cmd.Args, "-n", namespace)

	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get job status: %s", string(output))
	}
	outputStr := strings.Trim(string(output), "'")
	if outputStr != "1" {
		return fmt.Errorf("job %s/%s didn't succeed yet. .status.succeeded: %s", namespace, name, string(output))
	}
	return nil
}

func Logs(opts Options, args ...string) (string, error) {
	cmd := exec.Command("kubectl", "logs")
	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}
	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}
	cmd.Args = append(cmd.Args, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	fmt.Println(cmd)

	err := cmd.Run()

	if logOutput || err != nil {
		fmt.Println(stderr.String())
		fmt.Println(stdout.String())
	}

	if err != nil {
		return stdout.String() + stderr.String(), err
	}
	return stdout.String(), nil
}
