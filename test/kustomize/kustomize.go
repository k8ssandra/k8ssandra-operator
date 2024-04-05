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

func BuildDir(dir string) (*bytes.Buffer, error) {
	binDir := os.Getenv("LOCALBIN")
	kustomizeLocation := ""
	if binDir == "" {
		fmt.Println("warning: LOCALBIN environment variable not set, attempting to use system kustomize")
		kustomizeLocation = "kustomize"
	} else {
		fmt.Println("LOCALBIN: " + binDir)
		kustomizeLocation = binDir + "/kustomize"
	}
	cmd := exec.Command(kustomizeLocation, "build")
	cmd.Dir = dir

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()
	buffer := bytes.NewBuffer(output)

	if err != nil || logOutput {
		fmt.Println(string(output))
	}

	return buffer, err
}

func BuildUrl(url string) (*bytes.Buffer, error) {
	cmd := exec.Command("kustomize", "build", url)

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()
	buffer := bytes.NewBuffer(output)

	if err != nil || logOutput {
		fmt.Println(string(output))
	}

	return buffer, err
}

func SetNamespace(dir, namespace string) error {
	cmd := exec.Command("kustomize", "edit", "set", "namespace", namespace)
	cmd.Dir = dir

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}

func AddResource(path string) error {
	cmd := exec.Command("kustomize", "edit", "add", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}

func RemoveResource(path string) error {
	cmd := exec.Command("kustomize", "edit", "remove", "resource", path)
	cmd.Dir = "../testdata/k8ssandra-operator"

	fmt.Println(cmd)

	output, err := cmd.CombinedOutput()

	if logOutput {
		fmt.Println(string(output))
	}

	return err
}
