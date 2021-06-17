package kustomize

import (
	"bytes"
	"os/exec"
	"testing"
)

func Build(t *testing.T, dir string) (*bytes.Buffer, error) {
	cmd := exec.Command("kustomize", "build")
	cmd.Dir = dir

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()

	//t.Log(out.String())
	t.Log(stderr.String())

	return &out, err
}

func SetNamespace(t *testing.T, dir, namespace string) error {
	cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
	cmd.Dir = dir

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	t.Log(out.String())

	return err
}

func AddResource(t *testing.T, path string) error {
	cmd := exec.Command("kustomize" ,"edit", "add", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	t.Log(out.String())

	return err
}

func RemoveResource(t *testing.T, path string) error {
	cmd := exec.Command("kustomize" ,"edit", "remove", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	t.Log(out.String())

	return err
}
