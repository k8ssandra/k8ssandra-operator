package kustomize

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
)

func Build(dir string) (*bytes.Buffer, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	fmt.Println(stdout.String())
	fmt.Println(stderr.String())

	return &stdout, err
}

func SetNamespace(dir, namespace string) error {
	cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
	cmd.Dir = dir

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	fmt.Println(out.String())
	//t.Log(out.String())

	return err
}

func AddResource(t *testing.T, path string) error {
	cmd := exec.Command("kustomize", "edit", "add", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	t.Log(out.String())

	return err
}

func RemoveResource(t *testing.T, path string) error {
	cmd := exec.Command("kustomize", "edit", "remove", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	t.Log(out.String())

	return err
}
