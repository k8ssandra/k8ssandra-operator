package kustomize

import (
	"bytes"
	"fmt"
	"os/exec"
)

var logOutput = false

func LogOutput(enabled bool) {
	logOutput = enabled
}

func Build(dir string) (*bytes.Buffer, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	fmt.Println(cmd.String())

	err := cmd.Run()

	if logOutput {
		fmt.Println(stdout.String())
		fmt.Println(stderr.String())
	}

	return &stdout, err
}

func SetNamespace(dir, namespace string) error {
	cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
	cmd.Dir = dir

	var out bytes.Buffer
	cmd.Stdout = &out

	fmt.Println(cmd.String())

	err := cmd.Run()

	if logOutput {
		fmt.Println(out.String())
	}

	return err
}

func AddResource(path string) error {
	cmd := exec.Command("kustomize", "edit", "add", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	var out bytes.Buffer
	cmd.Stdout = &out

	fmt.Println(cmd.String())

	err := cmd.Run()

	if logOutput {
		fmt.Println(out.String())
	}

	return err
}

func RemoveResource(path string) error {
	cmd := exec.Command("kustomize", "edit", "remove", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	var out bytes.Buffer
	cmd.Stdout = &out

	fmt.Println(cmd.String())

	err := cmd.Run()

	if logOutput {
		fmt.Println(out.String())
	}

	return err
}
