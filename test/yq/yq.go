package yq

import (
	"fmt"
	"os/exec"
	"strings"
)

type Options struct {

	// All implies eval-all: the evaluation loads all yaml documents of all yaml files and runs the
	// expression once. If false (the default), the evaluation iterates over each yaml document from
	// each given file, applies the expression and prints the result in sequence.
	All bool

	// InPlace implies that, for mutating operations, mutations will happen in-place and modify the
	// original file.
	InPlace bool
}

// Eval runs yq and evaluates the given expression on the given files.
func Eval(expression string, options Options, files ...string) (string, error) {
	cmd := exec.Command("yq")
	if options.All {
		cmd.Args = append(cmd.Args, "ea")
	} else {
		cmd.Args = append(cmd.Args, "e")
	}
	cmd.Args = append(cmd.Args, expression)
	if options.InPlace {
		cmd.Args = append(cmd.Args, "-i")
	}
	cmd.Args = append(cmd.Args, files...)
	fmt.Println(cmd)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))
	if err != nil {
		return "", fmt.Errorf("eval failed: %s (%w)", result, err)
	}
	return result, nil
}
