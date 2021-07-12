package kubectl

import (
	"bytes"
	"fmt"
	"os/exec"
)

type Options struct {
	Namespace string
	Context   string
}

func Apply(opts Options, config fmt.Stringer) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args,"--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "apply", "-f")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if buf, ok := config.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else {
		cmd.Args = append(cmd.Args, config.String())
	}

	fmt.Println(cmd)

	err := cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())

	return err
}

func Delete(opts Options, config fmt.Stringer) error {
	cmd := exec.Command("kubectl")

	if len(opts.Context) > 0 {
		cmd.Args = append(cmd.Args,"--context", opts.Context)
	}

	if len(opts.Namespace) > 0 {
		cmd.Args = append(cmd.Args, "-n", opts.Namespace)
	}

	cmd.Args = append(cmd.Args, "delete", "-f")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if buf, ok := config.(*bytes.Buffer); ok {
		cmd.Stdin = buf
		cmd.Args = append(cmd.Args, "-")
	} else {
		cmd.Args = append(cmd.Args, config.String())
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

func DumpClusterInfo(namespace, outputDir string) error {
	args := []string{"cluster-info", "dump", "--namespaces", namespace, "-o", "yaml", "--output-directory", outputDir}
	cmd := exec.Command("kubectl", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())
	//t.Log(stdout.String())
	//t.Log(stderr.String())

	return err
}
