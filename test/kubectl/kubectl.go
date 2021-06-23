package kubectl

import (
	"bytes"
	"os/exec"
	"testing"
)

func ApplyBuffer(t *testing.T, b *bytes.Buffer) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stdin = b
	cmd.Stderr = &stderr

	err := cmd.Run()

	t.Log(stdout.String())
	t.Log(stderr.String())

	return err
}

func Apply(t *testing.T, namespace, path string) error {
	cmd := exec.Command("kubectl", "-n", namespace, "apply", "-f", path)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	t.Log(stdout.String())
	t.Log(stderr.String())

	return err
}

func DeleteBuffer(t *testing.T, b *bytes.Buffer) error {
	cmd := exec.Command("kubectl", "delete", "-f", "-")

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stdin = b
	cmd.Stderr = &stderr

	err := cmd.Run()

	t.Log(out.String())
	t.Log(stderr.String())

	return err
}

func WaitForCondition(t *testing.T, condition string, args ...string) error {
	kargs := []string{"wait", "--for", "condition=" + condition}
	kargs = append(kargs, args...)

	cmd := exec.Command("kubectl", kargs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	t.Log(stdout.String())
	t.Log(stderr.String())

	return err
}

func DumpClusterInfo(t *testing.T, namespace, outputDir string) error {
	args := []string{"cluster-info", "dump", "--namespaces", namespace, "-o", "yaml", "--output-directory", outputDir}
	cmd := exec.Command("kubectl", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	t.Log(stdout.String())
	t.Log(stderr.String())

	return err
}
