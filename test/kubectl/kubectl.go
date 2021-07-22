package kubectl

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"os/exec"
	"path/filepath"
)

type Options struct {
	Namespace string
	Context   string
}

func Apply(opts Options, arg interface{}) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "apply", "-f")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if buf, ok := arg.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else if s, ok := arg.(string); ok {
		cmd.Args = append(cmd.Args, s)
	} else {
		return errors.New("Expected arg to be a *bytes.Buffer or a string")
	}

	fmt.Println(cmd)

	err := cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())

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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if buf, ok := arg.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else if s, ok := arg.(string); ok {
		cmd.Args = append(cmd.Args, s)
	} else {
		return errors.New("Expected arg to be a *bytes.Buffer or a string")
	}

	fmt.Println(cmd)

	err := cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())

	return err
}

func WaitForCondition(condition string, args ...string) error {
	kargs := []string{"wait", "--for", "condition=" + condition}
	kargs = append(kargs, args...)

	cmd := exec.Command("kubectl", kargs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())

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
	if err != nil {
		return stderr.String(), err
	}

	return stdout.String(), nil
}

type ClusterInfoOptions struct {
	Options

	OutputDirectory string
}

func DumpClusterInfo(opts ClusterInfoOptions) error {
	cmd := exec.Command("kubectl", "cluster-info", "dump")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args, "--context", opts.Context)
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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())

	return err
}
