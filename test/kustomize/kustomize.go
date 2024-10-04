package kustomize

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

var logOutput = false

func LogOutput(enabled bool) {
	logOutput = enabled
}

func kustomizePath() string {
	binDir := os.Getenv("LOCALBIN")
	if binDir == "" {
		fmt.Println("warning: LOCALBIN environment variable not set, attempting to use system kustomize")
		return "kustomize"
	}
	return binDir + "/kustomize"
}

func BuildDir(dir string) (*bytes.Buffer, error) {
	cmd := exec.Command(kustomizePath(), "build")
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	buffer := bytes.NewBuffer(output)

	if err != nil || logOutput {
		fmt.Println(string(output))
	}

	return buffer, err
}

func BuildUrl(url string) (*bytes.Buffer, error) {
	cmd := exec.Command(kustomizePath(), "build", url)

	output, err := cmd.CombinedOutput()
	buffer := bytes.NewBuffer(output)

	if err != nil || logOutput {
		fmt.Println(string(output))
	}

	return buffer, err
}

func SetNamespace(dir, namespace string) error {
	cmd := exec.Command(kustomizePath(), "edit", "set", "namespace", namespace)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}

func AddResource(path string) error {
	cmd := exec.Command(kustomizePath(), "edit", "add", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}

func RemoveResource(path string) error {
	cmd := exec.Command(kustomizePath(), "edit", "remove", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}
